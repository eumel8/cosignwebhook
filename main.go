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
	"time"

	log "github.com/gookit/slog"

	"github.com/eumel8/cosignwebhook/webhook"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	port        = "8080"
	mport       = "8081"
	logTemplate = "[{{datetime}}] [{{level}}] {{caller}} {{message}} \n"
	timeout     = 10 * time.Second
)

var tlscert, tlskey string

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
		log.Errorf("failed to load key pair: %v", err)
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certs},
			MinVersion:   tls.VersionTLS12,
		},
		ReadHeaderTimeout: timeout,
	}

	mserver := &http.Server{
		Addr:              fmt.Sprintf(":%v", mport),
		ReadHeaderTimeout: timeout,
	}

	// define http server and server handler
	cs := webhook.NewCosignServerHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", cs.Serve)
	server.Handler = mux

	mmux := http.NewServeMux()
	mmux.HandleFunc("/healthz", cs.Healthz)
	mmux.Handle("/metrics", promhttp.Handler())
	mserver.Handler = mmux

	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()
	go func() {
		if err := mserver.ListenAndServe(); err != nil {
			log.Errorf("Failed to listen and serve monitor server: %v", err)
		}
	}()

	log.Info("Webhook server running", "port", port, "metricsPort", mport)

	// listening shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Info("Got shutdown signal, shutting down webhook server gracefully...")
	_ = server.Shutdown(context.Background())
	_ = mserver.Shutdown(context.Background())
}
