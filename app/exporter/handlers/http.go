// Package handlers provides event handlers for different bpftrace event types.
package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"fosdem2026/app/exporter/core"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// HTTPHandler handles HTTP request events from the native USDT instrumentation.
type HTTPHandler struct {
	spans *core.SpanManager
}

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(spans *core.SpanManager) *HTTPHandler {
	return &HTTPHandler{spans: spans}
}

// Name returns the handler name.
func (h *HTTPHandler) Name() string {
	return "http"
}

// CanHandle returns true for HTTP request events.
func (h *HTTPHandler) CanHandle(eventType string) bool {
	return eventType == "http_request_start" || eventType == "http_request_end"
}

// HandleStart processes an http_request_start event.
func (h *HTTPHandler) HandleStart(ctx context.Context, event map[string]any) error {
	e := core.Event(event)
	method := e.GetString("method")
	path := e.GetString("path")
	timestamp := e.GetInt64("timestamp")

	key := fmt.Sprintf("http:%s:%s:%d", method, path, timestamp)
	startTime := time.Unix(0, timestamp)

	_, span := h.spans.Tracer().Start(ctx, "HTTP "+method,
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(method),
			semconv.URLPath(path),
		),
	)

	h.spans.Store(key, &core.SpanContext{Span: span, StartTime: startTime})
	log.Printf("HTTP request started: %s %s", method, path)
	return nil
}

// HandleEnd processes an http_request_end event.
func (h *HTTPHandler) HandleEnd(event map[string]any) error {
	e := core.Event(event)
	method := e.GetString("method")
	path := e.GetString("path")
	status := e.GetInt64("status")
	duration := e.GetInt64("duration")

	spanCtx, _, err := h.spans.FindByPrefix("http:")
	if err != nil {
		return fmt.Errorf("no active span found for HTTP %s %s: %w", method, path, err)
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		semconv.HTTPResponseStatusCode(int(status)),
		attribute.Int64("duration_ns", duration),
		attribute.Float64("duration_ms", float64(duration)/1e6),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	log.Printf("HTTP request ended: %s %s status=%d duration=%.2fms", method, path, status, float64(duration)/1e6)
	return nil
}
