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

	"github.com/gookit/slog"

	"github.com/eumel8/cosignwebhook/webhook"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	port        = "8080"
	mport       = "8081"
	logTemplate = "[{{datetime}}] [{{level}}] {{caller}} {{message}} \n"
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
		slog.SetLogLevel(slog.FatalLevel)
	case "trace":
		slog.SetLogLevel(slog.TraceLevel)
	case "debug":
		slog.SetLogLevel(slog.DebugLevel)
	case "error":
		slog.SetLogLevel(slog.ErrorLevel)
	case "warn":
		slog.SetLogLevel(slog.WarnLevel)
	case "info":
		slog.SetLogLevel(slog.InfoLevel)
	default:
		slog.SetLogLevel(slog.InfoLevel)
	}

	slog.GetFormatter().(*slog.TextFormatter).SetTemplate(logTemplate)

	certs, err := tls.LoadX509KeyPair(tlscert, tlskey)
	if err != nil {
		slog.Error("failed to load key pair", "error", err)
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certs},
			MinVersion:   tls.VersionTLS12,
		},
		ReadHeaderTimeout: 10 * time.Second,
	}

	mserver := &http.Server{
		Addr:              fmt.Sprintf(":%v", mport),
		ReadHeaderTimeout: 10 * time.Second,
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

	// start webhook server in new rountine
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			slog.Error("Failed to listen and serve webhook server", "error", err)
		}
	}()
	go func() {
		if err := mserver.ListenAndServe(); err != nil {
			slog.Error("Failed to listen and serve monitor server %v", "error", err)
		}
	}()

	slog.Info("Webhook server running", "port", port, "metricsPort", mport)

	// listening shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	slog.Info("Got shutdown signal, shutting down webhook server gracefully...")
	_ = server.Shutdown(context.Background())
	_ = mserver.Shutdown(context.Background())
}
