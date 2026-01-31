package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"fosdem2026/app/exporter/core"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// RequestHandler handles generic request events from libstabst (salp) instrumentation.
type RequestHandler struct {
	spans *core.SpanManager
}

// NewRequestHandler creates a new RequestHandler.
func NewRequestHandler(spans *core.SpanManager) *RequestHandler {
	return &RequestHandler{spans: spans}
}

// Name returns the handler name.
func (h *RequestHandler) Name() string {
	return "request"
}

// CanHandle returns true for generic request events.
func (h *RequestHandler) CanHandle(eventType string) bool {
	return eventType == "request_start" || eventType == "request_end"
}

// HandleStart processes a request_start event.
func (h *RequestHandler) HandleStart(ctx context.Context, event map[string]any) error {
	e := core.Event(event)
	reqID := e.GetString("reqid")
	if reqID == "" {
		return fmt.Errorf("missing reqid")
	}

	timestamp := e.GetFloat64("timestamp")
	if timestamp == 0 {
		return fmt.Errorf("missing timestamp")
	}

	startTime := time.Unix(0, int64(timestamp))

	_, span := h.spans.Tracer().Start(ctx, "http.request",
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithAttributes(
			attribute.String("request.id", reqID),
			attribute.String("span.kind", "server"),
		),
	)

	h.spans.Store(reqID, &core.SpanContext{Span: span, StartTime: startTime})
	log.Printf("Started span for request: %s", reqID)
	return nil
}

// HandleEnd processes a request_end event.
func (h *RequestHandler) HandleEnd(event map[string]any) error {
	e := core.Event(event)
	reqID := e.GetString("reqid")
	if reqID == "" {
		return fmt.Errorf("missing reqid")
	}

	spanCtx, ok := h.spans.Remove(reqID)
	if !ok {
		return fmt.Errorf("no active span found for request: %s", reqID)
	}

	duration := e.GetFloat64("duration")
	if duration == 0 {
		return fmt.Errorf("missing duration")
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		attribute.Int64("duration_ns", int64(duration)),
		attribute.Float64("duration_ms", duration/1e6),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	log.Printf("Ended span for request: %s (duration: %.2fms)", reqID, duration/1e6)
	return nil
}
