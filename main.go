package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/gookit/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	port        = "8080"
	mport       = "8081"
	logTemplate = "[{{datetime}}] [{{level}}] {{caller}} {{message}} \n"
)

var (
	tlscert, tlskey string
	opsProcessed    = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cosign_processed_ops_total",
		Help: "The total number of processed events",
	})
	verifiedProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cosign_processed_verified_total",
		Help: "The number of verfified events",
	})
)

func main() {
	// parse arguments
	flag.StringVar(&tlscert, "tlsCertFile", "/etc/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&tlskey, "tlsKeyFile", "/etc/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	logLevel := flag.String("logLevel", "info", "loglevel of app, e.g info, debug, warn, error, fatal")
	flag.Parse()

	// set log level
	switch *logLevel {
	case "fatal":
		log.SetLogLevel(log.FatalLevel)
	case "trace":
		log.SetLogLevel(log.TraceLevel)
	case "debug":
		log.SetLogLevel(log.DebugLevel)
	case "error":
		log.SetLogLevel(log.ErrorLevel)
	case "warn":
		log.SetLogLevel(log.WarnLevel)
	case "info":
		log.SetLogLevel(log.InfoLevel)
	default:
		log.SetLogLevel(log.InfoLevel)
	}

	log.GetFormatter().(*log.TextFormatter).SetTemplate(logTemplate)

	certs, err := tls.LoadX509KeyPair(tlscert, tlskey)

	if err != nil {
		log.Errorf("Failed to load key pair: ", err)
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{certs}},
	}

	mserver := &http.Server{
		Addr: fmt.Sprintf(":%v", mport),
	}

	// define http server and server handler
	cs := NewCosignServerHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", cs.serve)
	server.Handler = mux

	mmux := http.NewServeMux()
	mmux.HandleFunc("/healthz", cs.healthz)
	mmux.Handle("/metrics", promhttp.Handler())
	mserver.Handler = mmux

	// start webhook server in new rountine
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Errorf("Failed to listen and serve webhook server ", err)
		}
	}()
	go func() {
		if err := mserver.ListenAndServe(); err != nil {
			log.Errorf("Failed to listen and serve minitor server ", err)
		}
	}()

	log.Infof("Server running listening in port: %s,%s", port, mport)

	// listening shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Info("Got shutdown signal, shutting down webhook server gracefully...")
	server.Shutdown(context.Background())
	mserver.Shutdown(context.Background())
}
