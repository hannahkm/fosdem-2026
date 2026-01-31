// Package core provides the shared types and functionality for the bpftrace exporter.
package core

import (
	"context"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// SpanContext tracks active spans by a composite key.
type SpanContext struct {
	Span      oteltrace.Span
	StartTime time.Time
}

// EventHandler processes bpftrace events and manages span lifecycle.
type EventHandler interface {
	// CanHandle returns true if this handler can process the given event type.
	CanHandle(eventType string) bool

	// HandleStart processes a start event and creates a span.
	HandleStart(ctx context.Context, event map[string]any) error

	// HandleEnd processes an end event and completes a span.
	HandleEnd(event map[string]any) error

	// Name returns the handler name for logging.
	Name() string
}

// Event represents a parsed bpftrace event.
type Event map[string]any

// GetString extracts a string value from the event.
func (e Event) GetString(key string) string {
	if v, ok := e[key].(string); ok {
		return v
	}
	return ""
}

// GetInt64 extracts an int64 value from the event (JSON numbers are float64).
func (e Event) GetInt64(key string) int64 {
	if v, ok := e[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// GetFloat64 extracts a float64 value from the event.
func (e Event) GetFloat64(key string) float64 {
	if v, ok := e[key].(float64); ok {
		return v
	}
	return 0
}
