// Command fortiadc-exporter serves Prometheus metrics scraped from FortiADC
// appliances' REST monitor API.
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/config"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/fortiadc"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/handler"
	"gitlab.com/endekasoft/fortiadc-exporter/internal/metrics"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	configPath := flag.String("config", "config.yml", "path to the exporter's config.yml")
	flag.Parse()

	log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Str("event", "config_load_failed").Err(err).Msg("failed to load config")
	}

	metrics.Register(version)
	targets := make(map[string]*handler.Target, len(cfg.FortiADC))
	for name, t := range cfg.FortiADC {
		client := fortiadc.NewClient(t.URL, t.APIKey, t.VDOM, t.InsecureSkipVerify)
		client.Observe = metrics.ObserveAPI
		targets[name] = &handler.Target{API: client, VDOM: t.VDOM}
	}

	mux := http.NewServeMux()
	mux.Handle(handler.ProbePath, handler.NewProbe(targets))
	mux.Handle(handler.MetricsPath, promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", handler.Index)

	srv := &http.Server{Addr: cfg.ListenAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Info().Str("event", "fortiadc_exporter_starting").Str("version", version).
		Str("listen_address", cfg.ListenAddress).Int("target_count", len(targets)).Msg("starting fortiadc-exporter")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Str("event", "http_server_failed").Err(err).Msg("http server stopped")
	}
	log.Info().Str("event", "fortiadc_exporter_stopped").Msg("fortiadc-exporter stopped")
}
