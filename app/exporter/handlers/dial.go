package handlers

import (
	"context"
	"fmt"
	"time"

	"fosdem2026/app/exporter/core"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// DialHandler handles network dial events from the native USDT instrumentation.
type DialHandler struct {
	spans *core.SpanManager
}

// NewDialHandler creates a new DialHandler.
func NewDialHandler(spans *core.SpanManager) *DialHandler {
	return &DialHandler{spans: spans}
}

// Name returns the handler name.
func (h *DialHandler) Name() string {
	return "dial"
}

// CanHandle returns true for network dial events.
func (h *DialHandler) CanHandle(eventType string) bool {
	return eventType == "net_dial_start" || eventType == "net_dial_end"
}

// HandleStart processes a net_dial_start event.
func (h *DialHandler) HandleStart(ctx context.Context, event map[string]any) error {
	e := core.Event(event)
	network := e.GetString("network")
	address := e.GetString("address")
	timestamp := e.GetInt64("timestamp")

	key := fmt.Sprintf("dial:%s:%s:%d", network, address, timestamp)
	startTime := time.Unix(0, timestamp)

	_, span := h.spans.Tracer().Start(ctx, "net.Dial",
		oteltrace.WithTimestamp(startTime),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("net.transport", network),
			attribute.String("net.peer.name", address),
		),
	)

	h.spans.Store(key, &core.SpanContext{Span: span, StartTime: startTime})
	return nil
}

// HandleEnd processes a net_dial_end event.
func (h *DialHandler) HandleEnd(event map[string]any) error {
	e := core.Event(event)
	network := e.GetString("network")
	address := e.GetString("address")
	duration := e.GetInt64("duration")
	errCode := e.GetInt64("error")

	spanCtx, _, err := h.spans.FindByPrefix("dial:")
	if err != nil {
		return fmt.Errorf("no active span found for dial %s %s: %w", network, address, err)
	}

	endTime := spanCtx.StartTime.Add(time.Duration(duration))

	spanCtx.Span.SetAttributes(
		attribute.Int64("duration_ns", duration),
		attribute.Bool("error", errCode != 0),
	)

	spanCtx.Span.End(oteltrace.WithTimestamp(endTime))
	return nil
}
