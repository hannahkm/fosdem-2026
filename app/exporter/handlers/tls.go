package handlers

import (
	"context"
	"fmt"
	"time"

	"fosdem2026/app/exporter/core"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TLSHandler handles TLS handshake events from the native USDT instrumentation.
type TLSHandler struct {
	spans *core.SpanManager
}

// NewTLSHandler creates a new TLSHandler.
func NewTLSHandler(spans *core.SpanManager) *TLSHandler {
	return &TLSHandler{spans: spans}
}

// Name returns the handler name.
func (h *TLSHandler) Name() string {
	return "tls"
}

// CanHandle returns true for TLS handshake events.
func (h *TLSHandler) CanHandle(eventType string) bool {
	return eventType == "tls_handshake_start" || eventType == "tls_handshake_end"
}

// HandleStart processes a tls_handshake_start event.
func (h *TLSHandler) HandleStart(ctx context.Context, event map[string]any) error {
	e := core.Event(event)
	serverName := e.GetString("server_name")
	timestamp := e.GetInt64("timestamp")

	key := fmt.Sprintf("tls:%s:%d", serverName, timestamp)
	startTime := time.Unix(0, timestamp)

	_, span := h.spans.Tracer().Start(ctx, "tls.Handshake",
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("tls.server_name", serverName),
		),
	)

	h.spans.Store(key, &core.SpanContext{Span: span, StartTime: startTime})
	return nil
}

// HandleEnd processes a tls_handshake_end event.
func (h *TLSHandler) HandleEnd(event map[string]any) error {
	e := core.Event(event)
	serverName := e.GetString("server_name")
	duration := e.GetInt64("duration")
	errCode := e.GetInt64("error")

	spanCtx, _, err := h.spans.FindByPrefix("tls:")
	if err != nil {
		return fmt.Errorf("no active span found for TLS handshake %s: %w", serverName, err)
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		attribute.Int64("duration_ns", duration),
		attribute.Bool("error", errCode != 0),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	return nil
}
