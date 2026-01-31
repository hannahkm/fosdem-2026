package core

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func setupTestExporter(t *testing.T) (*Exporter, func()) {
	t.Helper()

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	config := &Config{
		Mode:         ModeLibstabst,
		OTELEndpoint: "localhost:4318",
		TargetPID:    "1",
		BPFScript:    "/app/trace-json.bt",
		ServiceName:  "test-exporter",
		TracerName:   "test-exporter",
	}

	exporter := New(config)
	exporter.spans = NewSpanManager(otel.Tracer("test-exporter"))

	return exporter, func() {
		_ = tp.Shutdown(context.Background())
	}
}

type mockHandler struct {
	name       string
	canHandle  func(string) bool
	startCalls []map[string]any
	endCalls   []map[string]any
}

func (h *mockHandler) Name() string { return h.name }

func (h *mockHandler) CanHandle(eventType string) bool {
	if h.canHandle != nil {
		return h.canHandle(eventType)
	}
	return false
}

func (h *mockHandler) HandleStart(_ context.Context, event map[string]any) error {
	h.startCalls = append(h.startCalls, event)
	return nil
}

func (h *mockHandler) HandleEnd(event map[string]any) error {
	h.endCalls = append(h.endCalls, event)
	return nil
}

func TestExporter_RegisterHandler(t *testing.T) {
	exporter, cleanup := setupTestExporter(t)
	defer cleanup()

	handler := &mockHandler{name: "test"}
	exporter.RegisterHandler(handler)

	if len(exporter.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(exporter.handlers))
	}
}

func TestExporter_ProcessLine_RoutesToCorrectHandler(t *testing.T) {
	exporter, cleanup := setupTestExporter(t)
	defer cleanup()

	httpHandler := &mockHandler{
		name:      "http",
		canHandle: func(et string) bool { return et == "http_request_start" || et == "http_request_end" },
	}
	requestHandler := &mockHandler{
		name:      "request",
		canHandle: func(et string) bool { return et == "request_start" || et == "request_end" },
	}

	exporter.RegisterHandler(httpHandler)
	exporter.RegisterHandler(requestHandler)

	ctx := context.Background()

	// Test HTTP start event
	err := exporter.ProcessLine(ctx, `{"event":"http_request_start","method":"GET","path":"/api"}`)
	if err != nil {
		t.Fatalf("ProcessLine error: %v", err)
	}
	if len(httpHandler.startCalls) != 1 {
		t.Errorf("expected 1 HTTP start call, got %d", len(httpHandler.startCalls))
	}

	// Test request start event
	err = exporter.ProcessLine(ctx, `{"event":"request_start","reqid":"req-001","timestamp":1700000000000000000}`)
	if err != nil {
		t.Fatalf("ProcessLine error: %v", err)
	}
	if len(requestHandler.startCalls) != 1 {
		t.Errorf("expected 1 request start call, got %d", len(requestHandler.startCalls))
	}

	// Test HTTP end event
	err = exporter.ProcessLine(ctx, `{"event":"http_request_end","method":"GET","path":"/api","status":200}`)
	if err != nil {
		t.Fatalf("ProcessLine error: %v", err)
	}
	if len(httpHandler.endCalls) != 1 {
		t.Errorf("expected 1 HTTP end call, got %d", len(httpHandler.endCalls))
	}
}

func TestExporter_ProcessLine_SkipsNonEventLines(t *testing.T) {
	exporter, cleanup := setupTestExporter(t)
	defer cleanup()

	handler := &mockHandler{
		name:      "test",
		canHandle: func(string) bool { return true },
	}
	exporter.RegisterHandler(handler)

	ctx := context.Background()

	// bpftrace metadata line (no event field)
	err := exporter.ProcessLine(ctx, `{"type":"attached_probes","count":6}`)
	if err != nil {
		t.Fatalf("ProcessLine error: %v", err)
	}

	if len(handler.startCalls)+len(handler.endCalls) != 0 {
		t.Error("expected no handler calls for non-event lines")
	}
}

func TestExporter_ProcessLine_InvalidJSON(t *testing.T) {
	exporter, cleanup := setupTestExporter(t)
	defer cleanup()

	ctx := context.Background()

	err := exporter.ProcessLine(ctx, `{invalid json}`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestIsEndEvent(t *testing.T) {
	tests := []struct {
		eventType string
		expected  bool
	}{
		{"http_request_start", false},
		{"http_request_end", true},
		{"net_dial_start", false},
		{"net_dial_end", true},
		{"tls_handshake_start", false},
		{"tls_handshake_end", true},
		{"request_start", false},
		{"request_end", true},
		{"operation_done", true},
		{"operation_complete", true},
		{"operation_finish", true},
		{"start", false},
		{"end", false}, // Too short
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := isEndEvent(tt.eventType)
			if result != tt.expected {
				t.Errorf("isEndEvent(%q) = %v, want %v", tt.eventType, result, tt.expected)
			}
		})
	}
}
