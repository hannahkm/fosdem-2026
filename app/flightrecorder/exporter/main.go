// Package main implements a FlightRecorder trace to OTLP exporter sidecar.
// It watches for trace files, converts them to OTLP spans, and exports to a collector.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Get configuration from environment
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector:4318"
	}
	traceDir := os.Getenv("TRACE_OUTPUT_DIR")
	if traceDir == "" {
		traceDir = "/tmp/traces"
	}

	log.Printf("Starting FlightRecorder exporter")
	log.Printf("OTLP endpoint: %s", endpoint)
	log.Printf("Trace directory: %s", traceDir)

	// Initialize OTLP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("Failed to create OTLP exporter: %v", err)
	}

	// Create tracer provider
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("flightrecorder"),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	otel.SetTracerProvider(tp)

	// Create trace directory if it doesn't exist
	if err := os.MkdirAll(traceDir, 0755); err != nil {
		log.Fatalf("Failed to create trace directory: %v", err)
	}

	// Start file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(traceDir); err != nil {
		log.Fatalf("Failed to watch directory: %v", err)
	}

	log.Printf("Watching for trace files in %s", traceDir)

	// Process existing files first
	entries, err := os.ReadDir(traceDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".trace") {
				filePath := filepath.Join(traceDir, entry.Name())
				log.Printf("Processing existing trace file: %s", filePath)
				if err := processTraceFile(ctx, filePath); err != nil {
					log.Printf("Error processing %s: %v", filePath, err)
				}
			}
		}
	}

	// Watch for new files
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down")
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				if strings.HasSuffix(event.Name, ".trace") {
					log.Printf("New trace file detected: %s", event.Name)
					// Wait a bit for the file to be fully written
					time.Sleep(500 * time.Millisecond)
					if err := processTraceFile(ctx, event.Name); err != nil {
						log.Printf("Error processing %s: %v", event.Name, err)
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// processTraceFile reads a FlightRecorder trace file and converts it to OTLP spans.
func processTraceFile(ctx context.Context, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Printf("Converting trace file: %s", filePath)
	if err := convertTraceToSpans(ctx, f); err != nil {
		return err
	}

	log.Printf("Successfully processed: %s", filePath)
	return nil
}
