// Package main provides a bpftrace to OpenTelemetry exporter bridge.
// It runs bpftrace against a Go application built with the native USDT fork,
// captures stdlib probe events, and exports them as OTLP traces.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// SpanContext tracks active spans by a composite key
type SpanContext struct {
	Span      oteltrace.Span
	StartTime time.Time
}

var (
	tracer       oteltrace.Tracer
	activeSpans  = make(map[string]*SpanContext)
	spansMu      sync.Mutex
	otelEndpoint string
	targetPID    string
	bpfScript    string
)

func init() {
	otelEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "otel-collector:4318"
	}

	targetPID = os.Getenv("TARGET_PID")
	if targetPID == "" {
		targetPID = "1"
	}

	bpfScript = os.Getenv("BPFTRACE_SCRIPT")
	if bpfScript == "" {
		bpfScript = "/app/trace-json.bt"
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown, err := initTracer(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	tracer = otel.Tracer("usdt-native-exporter")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, cleaning up...")
		cancel()
	}()

	if err := runBPFTrace(ctx); err != nil {
		log.Fatalf("BPFTrace error: %v", err)
	}

	log.Println("Exporter shutting down")
}

func initTracer(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(otelEndpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("usdt-native-exporter"),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("exporter.type", "bpftrace"),
			attribute.String("instrumentation.type", "native-usdt"),
		),
	)
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

func runBPFTrace(ctx context.Context) error {
	log.Printf("Starting bpftrace with script: %s, target PID: %s", bpfScript, targetPID)

	cmd := exec.CommandContext(ctx, "bpftrace", "-f", "json", "-p", targetPID, bpfScript)
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

		if err := processEvent(ctx, line); err != nil {
			log.Printf("Warning: Failed to process event: %v", err)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading bpftrace output: %w", err)
	}

	return cmd.Wait()
}

func processEvent(ctx context.Context, jsonLine string) error {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	eventType, ok := event["event"].(string)
	if !ok {
		return nil // Skip non-event lines (bpftrace metadata)
	}

	switch eventType {
	case "http_request_start":
		return handleHTTPRequestStart(ctx, event)
	case "http_request_end":
		return handleHTTPRequestEnd(event)
	case "net_dial_start":
		return handleNetDialStart(ctx, event)
	case "net_dial_end":
		return handleNetDialEnd(event)
	case "tls_handshake_start":
		return handleTLSHandshakeStart(ctx, event)
	case "tls_handshake_end":
		return handleTLSHandshakeEnd(event)
	default:
		log.Printf("Unknown event type: %s", eventType)
		return nil
	}
}

func handleHTTPRequestStart(ctx context.Context, event map[string]interface{}) error {
	method := getString(event, "method")
	path := getString(event, "path")
	timestamp := getInt64(event, "timestamp")

	key := fmt.Sprintf("http:%s:%s:%d", method, path, timestamp)
	startTime := time.Unix(0, timestamp)

	_, span := tracer.Start(ctx, "HTTP "+method,
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(method),
			semconv.URLPath(path),
		),
	)

	spansMu.Lock()
	activeSpans[key] = &SpanContext{Span: span, StartTime: startTime}
	spansMu.Unlock()

	log.Printf("HTTP request started: %s %s", method, path)
	return nil
}

func handleHTTPRequestEnd(event map[string]interface{}) error {
	method := getString(event, "method")
	path := getString(event, "path")
	status := getInt64(event, "status")
	duration := getInt64(event, "duration")

	spansMu.Lock()
	defer spansMu.Unlock()

	// Find matching span by method and path
	var matchedKey string
	var spanCtx *SpanContext
	for key, ctx := range activeSpans {
		if len(key) > 5 && key[:5] == "http:" {
			spanCtx = ctx
			matchedKey = key
			break
		}
	}

	if spanCtx == nil {
		return fmt.Errorf("no active span found for HTTP %s %s", method, path)
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		semconv.HTTPResponseStatusCode(int(status)),
		attribute.Int64("duration_ns", duration),
		attribute.Float64("duration_ms", float64(duration)/1e6),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	delete(activeSpans, matchedKey)

	log.Printf("HTTP request ended: %s %s status=%d duration=%.2fms", method, path, status, float64(duration)/1e6)
	return nil
}

func handleNetDialStart(ctx context.Context, event map[string]interface{}) error {
	network := getString(event, "network")
	address := getString(event, "address")
	timestamp := getInt64(event, "timestamp")

	key := fmt.Sprintf("dial:%s:%s:%d", network, address, timestamp)
	startTime := time.Unix(0, timestamp)

	_, span := tracer.Start(ctx, "net.Dial",
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("net.transport", network),
			attribute.String("net.peer.name", address),
		),
	)

	spansMu.Lock()
	activeSpans[key] = &SpanContext{Span: span, StartTime: startTime}
	spansMu.Unlock()

	return nil
}

func handleNetDialEnd(event map[string]interface{}) error {
	network := getString(event, "network")
	address := getString(event, "address")
	duration := getInt64(event, "duration")
	errCode := getInt64(event, "error")

	spansMu.Lock()
	defer spansMu.Unlock()

	var matchedKey string
	var spanCtx *SpanContext
	for key, ctx := range activeSpans {
		if len(key) > 5 && key[:5] == "dial:" {
			spanCtx = ctx
			matchedKey = key
			break
		}
	}

	if spanCtx == nil {
		return fmt.Errorf("no active span found for dial %s %s", network, address)
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		attribute.Int64("duration_ns", duration),
		attribute.Bool("error", errCode != 0),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	delete(activeSpans, matchedKey)

	return nil
}

func handleTLSHandshakeStart(ctx context.Context, event map[string]interface{}) error {
	serverName := getString(event, "server_name")
	timestamp := getInt64(event, "timestamp")

	key := fmt.Sprintf("tls:%s:%d", serverName, timestamp)
	startTime := time.Unix(0, timestamp)

	_, span := tracer.Start(ctx, "tls.Handshake",
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("tls.server_name", serverName),
		),
	)

	spansMu.Lock()
	activeSpans[key] = &SpanContext{Span: span, StartTime: startTime}
	spansMu.Unlock()

	return nil
}

func handleTLSHandshakeEnd(event map[string]interface{}) error {
	serverName := getString(event, "server_name")
	duration := getInt64(event, "duration")
	errCode := getInt64(event, "error")

	spansMu.Lock()
	defer spansMu.Unlock()

	var matchedKey string
	var spanCtx *SpanContext
	for key, ctx := range activeSpans {
		if len(key) > 4 && key[:4] == "tls:" {
			spanCtx = ctx
			matchedKey = key
			break
		}
	}

	if spanCtx == nil {
		return fmt.Errorf("no active span found for TLS handshake %s", serverName)
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		attribute.Int64("duration_ns", duration),
		attribute.Bool("error", errCode != 0),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	delete(activeSpans, matchedKey)

	return nil
}

func getString(event map[string]interface{}, key string) string {
	if v, ok := event[key].(string); ok {
		return v
	}
	return ""
}

func getInt64(event map[string]interface{}, key string) int64 {
	if v, ok := event[key].(float64); ok {
		return int64(v)
	}
	return 0
}
