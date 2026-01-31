// Package main provides the unified bpftrace to OpenTelemetry exporter.
// It supports multiple instrumentation modes via the EXPORTER_MODE environment variable.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"fosdem2026/app/exporter/core"
	"fosdem2026/app/exporter/handlers"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := core.DefaultConfig()
	config.LoadFromEnv()

	exporter := core.New(config)

	if err := exporter.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}
	defer func() {
		if err := exporter.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down exporter: %v", err)
		}
	}()

	registerHandlers(exporter, config.Mode)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, cleaning up...")
		cancel()
	}()

	log.Printf("Starting exporter in %s mode", config.Mode)
	if err := exporter.Run(ctx); err != nil {
		log.Fatalf("Exporter error: %v", err)
	}

	log.Println("Exporter shutting down")
}

func registerHandlers(exporter *core.Exporter, mode core.Mode) {
	spans := exporter.SpanManager()

	switch mode {
	case core.ModeNativeUSDT:
		exporter.RegisterHandler(handlers.NewHTTPHandler(spans))
		exporter.RegisterHandler(handlers.NewDialHandler(spans))
		exporter.RegisterHandler(handlers.NewTLSHandler(spans))
		log.Println("Registered handlers: http, dial, tls")
	case core.ModeLibstabst:
		exporter.RegisterHandler(handlers.NewRequestHandler(spans))
		log.Println("Registered handlers: request")
	default:
		// Default: register all handlers
		exporter.RegisterHandler(handlers.NewHTTPHandler(spans))
		exporter.RegisterHandler(handlers.NewDialHandler(spans))
		exporter.RegisterHandler(handlers.NewTLSHandler(spans))
		exporter.RegisterHandler(handlers.NewRequestHandler(spans))
		log.Println("Registered handlers: http, dial, tls, request")
	}
}
