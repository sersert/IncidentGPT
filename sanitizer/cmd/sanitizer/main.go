package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"incidentgpt-sanitizer/internal/api"
	"incidentgpt-sanitizer/internal/auth"
	"incidentgpt-sanitizer/internal/config"
	"incidentgpt-sanitizer/internal/detector"
	"incidentgpt-sanitizer/internal/masking"
	"incidentgpt-sanitizer/internal/metrics"
	"incidentgpt-sanitizer/internal/policy"
	"incidentgpt-sanitizer/internal/pseudonymizer"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, loadErr := config.Load()
	readyErr := loadErr
	if readyErr == nil {
		readyErr = cfg.Validate()
	}
	if !cfg.FailClosed {
		log.Printf("WARN: SANITIZER_FAIL_CLOSED=false is intended only for local development")
	}

	d, err := detector.New(cfg.CustomPatterns)
	if readyErr == nil && err != nil {
		readyErr = err
	}
	pe := policy.NewEngine(cfg)
	pseudo := pseudonymizer.New(cfg.HashKey)
	m := metrics.New()
	engine := masking.NewEngine(cfg, d, pe, pseudo)
	server := api.New(cfg, engine, auth.NewVerifier(cfg.AuthSharedSecret, cfg.AuthMaxClockSkew), m, readyErr)

	srv := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("INFO: received signal %v, shutting down sanitizer", sig)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("ERROR: sanitizer shutdown: %v", err)
		}
	}()

	log.Printf("incidentgpt-sanitizer starting on %s", cfg.ListenAddress)
	if readyErr != nil {
		log.Printf("ERROR: sanitizer not ready: %v", readyErr)
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
