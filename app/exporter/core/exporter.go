package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Exporter is the main bpftrace to OpenTelemetry exporter.
type Exporter struct {
	config   *Config
	handlers []EventHandler
	spans    *SpanManager
	shutdown func(context.Context) error
}

// New creates a new Exporter with the given configuration.
func New(config *Config) *Exporter {
	return &Exporter{
		config:   config,
		handlers: make([]EventHandler, 0),
	}
}

// RegisterHandler adds an event handler to the exporter.
func (e *Exporter) RegisterHandler(h EventHandler) {
	e.handlers = append(e.handlers, h)
}

// SpanManager returns the span manager for handlers to use.
func (e *Exporter) SpanManager() *SpanManager {
	return e.spans
}

// Init initializes the OpenTelemetry tracer and span manager.
func (e *Exporter) Init(ctx context.Context) error {
	shutdown, err := e.initTracer(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize tracer: %w", err)
	}
	e.shutdown = shutdown

	tracer := otel.Tracer(e.config.TracerName)
	e.spans = NewSpanManager(tracer)

	return nil
}

// Shutdown cleanly shuts down the exporter.
func (e *Exporter) Shutdown(ctx context.Context) error {
	if e.shutdown != nil {
		return e.shutdown(ctx)
	}
	return nil
}

// Run starts the bpftrace process and processes events.
func (e *Exporter) Run(ctx context.Context) error {
	log.Printf("Starting bpftrace with script: %s, target PID: %s", e.config.BPFScript, e.config.TargetPID)

	cmd := exec.CommandContext(ctx, "bpftrace", "-f", "json", "-p", e.config.TargetPID, e.config.BPFScript)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start bpftrace: %w", err)
	}

	log.Println("BPFTrace started, processing events...")

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if err := e.processEvent(ctx, line); err != nil {
			log.Printf("Warning: Failed to process event: %v", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading bpftrace output: %w", err)
	}

	return cmd.Wait()
}

// ProcessLine processes a single JSON line (useful for testing).
func (e *Exporter) ProcessLine(ctx context.Context, line string) error {
	return e.processEvent(ctx, line)
}

func (e *Exporter) processEvent(ctx context.Context, jsonLine string) error {
	var event Event
	if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	eventType := event.GetString("event")
	if eventType == "" {
		// Skip non-event lines (bpftrace metadata)
		return nil
	}

	for _, h := range e.handlers {
		if h.CanHandle(eventType) {
			// Determine if this is a start or end event
			if isEndEvent(eventType) {
				return h.HandleEnd(event)
			}
			return h.HandleStart(ctx, event)
		}
	}

	log.Printf("Unknown event type: %s", eventType)
	return nil
}

func isEndEvent(eventType string) bool {
	suffixes := []string{"_end", "_done", "_complete", "_finish"}
	for _, suffix := range suffixes {
		if len(eventType) > len(suffix) && eventType[len(eventType)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

func (e *Exporter) initTracer(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(e.config.OTELEndpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName(e.config.ServiceName),
		semconv.ServiceVersion("1.0.0"),
		attribute.String("exporter.type", "bpftrace"),
	}

	if e.config.Mode == ModeNativeUSDT {
		attrs = append(attrs, attribute.String("instrumentation.type", "native-usdt"))
	}

	res, err := resource.New(ctx, resource.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
