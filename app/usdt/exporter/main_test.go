package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func setupTestTracer(t *testing.T) func() {
	t.Helper()

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer = otel.Tracer("test-usdt-exporter")

	spansMu.Lock()
	activeSpans = make(map[string]*SpanContext)
	spansMu.Unlock()

	return func() {
		_ = tp.Shutdown(context.Background())
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		event    map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "existing string key",
			event:    map[string]interface{}{"method": "GET"},
			key:      "method",
			expected: "GET",
		},
		{
			name:     "missing key",
			event:    map[string]interface{}{"method": "GET"},
			key:      "path",
			expected: "",
		},
		{
			name:     "non-string value",
			event:    map[string]interface{}{"status": 200},
			key:      "status",
			expected: "",
		},
		{
			name:     "empty string value",
			event:    map[string]interface{}{"path": ""},
			key:      "path",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.event, tt.key)
			if result != tt.expected {
				t.Errorf("getString(%v, %q) = %q, want %q", tt.event, tt.key, result, tt.expected)
			}
		})
	}
}

func TestGetInt64(t *testing.T) {
	tests := []struct {
		name     string
		event    map[string]interface{}
		key      string
		expected int64
	}{
		{
			name:     "existing float64 key",
			event:    map[string]interface{}{"status": float64(200)},
			key:      "status",
			expected: 200,
		},
		{
			name:     "missing key",
			event:    map[string]interface{}{"status": float64(200)},
			key:      "duration",
			expected: 0,
		},
		{
			name:     "non-float64 value",
			event:    map[string]interface{}{"method": "GET"},
			key:      "method",
			expected: 0,
		},
		{
			name:     "large timestamp",
			event:    map[string]interface{}{"timestamp": float64(1700000000000000000)},
			key:      "timestamp",
			expected: 1700000000000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInt64(tt.event, tt.key)
			if result != tt.expected {
				t.Errorf("getInt64(%v, %q) = %d, want %d", tt.event, tt.key, result, tt.expected)
			}
		})
	}
}

func TestProcessEvent_Routing(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	tests := []struct {
		name      string
		jsonLine  string
		wantErr   bool
		checkSpan bool
	}{
		{
			name:      "http_request_start",
			jsonLine:  `{"event":"http_request_start","method":"GET","path":"/api","timestamp":1700000000000000000}`,
			wantErr:   false,
			checkSpan: true,
		},
		{
			name:      "net_dial_start",
			jsonLine:  `{"event":"net_dial_start","network":"tcp","address":"localhost:5432","timestamp":1700000000000000000}`,
			wantErr:   false,
			checkSpan: true,
		},
		{
			name:      "tls_handshake_start",
			jsonLine:  `{"event":"tls_handshake_start","server_name":"example.com","timestamp":1700000000000000000}`,
			wantErr:   false,
			checkSpan: true,
		},
		{
			name:     "unknown event type",
			jsonLine: `{"event":"unknown_event","data":"test"}`,
			wantErr:  false,
		},
		{
			name:     "invalid json",
			jsonLine: `{invalid json}`,
			wantErr:  true,
		},
		{
			name:     "bpftrace metadata (no event field)",
			jsonLine: `{"type":"attached_probes","count":6}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spansMu.Lock()
			activeSpans = make(map[string]*SpanContext)
			spansMu.Unlock()

			err := processEvent(context.Background(), tt.jsonLine)
			if (err != nil) != tt.wantErr {
				t.Errorf("processEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkSpan {
				spansMu.Lock()
				spanCount := len(activeSpans)
				spansMu.Unlock()
				if spanCount == 0 {
					t.Error("expected span to be created, but activeSpans is empty")
				}
			}
		})
	}
}

func TestHTTPRequestLifecycle(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	timestamp := int64(1700000000000000000)

	startEvent := map[string]interface{}{
		"event":     "http_request_start",
		"method":    "POST",
		"path":      "/api/users",
		"timestamp": float64(timestamp),
	}

	err := handleHTTPRequestStart(ctx, startEvent)
	if err != nil {
		t.Fatalf("handleHTTPRequestStart() error = %v", err)
	}

	spansMu.Lock()
	if len(activeSpans) != 1 {
		t.Errorf("expected 1 active span, got %d", len(activeSpans))
	}
	spansMu.Unlock()

	endEvent := map[string]interface{}{
		"event":    "http_request_end",
		"method":   "POST",
		"path":     "/api/users",
		"status":   float64(201),
		"duration": float64(5000000),
	}

	err = handleHTTPRequestEnd(endEvent)
	if err != nil {
		t.Fatalf("handleHTTPRequestEnd() error = %v", err)
	}

	spansMu.Lock()
	if len(activeSpans) != 0 {
		t.Errorf("expected 0 active spans after end, got %d", len(activeSpans))
	}
	spansMu.Unlock()
}

func TestNetDialLifecycle(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	timestamp := int64(1700000000000000000)

	startEvent := map[string]interface{}{
		"event":     "net_dial_start",
		"network":   "tcp",
		"address":   "db.example.com:5432",
		"timestamp": float64(timestamp),
	}

	err := handleNetDialStart(ctx, startEvent)
	if err != nil {
		t.Fatalf("handleNetDialStart() error = %v", err)
	}

	spansMu.Lock()
	if len(activeSpans) != 1 {
		t.Errorf("expected 1 active span, got %d", len(activeSpans))
	}
	spansMu.Unlock()

	endEvent := map[string]interface{}{
		"event":    "net_dial_end",
		"network":  "tcp",
		"address":  "db.example.com:5432",
		"duration": float64(2000000),
		"error":    float64(0),
	}

	err = handleNetDialEnd(endEvent)
	if err != nil {
		t.Fatalf("handleNetDialEnd() error = %v", err)
	}

	spansMu.Lock()
	if len(activeSpans) != 0 {
		t.Errorf("expected 0 active spans after end, got %d", len(activeSpans))
	}
	spansMu.Unlock()
}

func TestTLSHandshakeLifecycle(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	timestamp := int64(1700000000000000000)

	startEvent := map[string]interface{}{
		"event":       "tls_handshake_start",
		"server_name": "secure.example.com",
		"timestamp":   float64(timestamp),
	}

	err := handleTLSHandshakeStart(ctx, startEvent)
	if err != nil {
		t.Fatalf("handleTLSHandshakeStart() error = %v", err)
	}

	spansMu.Lock()
	if len(activeSpans) != 1 {
		t.Errorf("expected 1 active span, got %d", len(activeSpans))
	}
	spansMu.Unlock()

	endEvent := map[string]interface{}{
		"event":       "tls_handshake_end",
		"server_name": "secure.example.com",
		"duration":    float64(3000000),
		"error":       float64(0),
	}

	err = handleTLSHandshakeEnd(endEvent)
	if err != nil {
		t.Fatalf("handleTLSHandshakeEnd() error = %v", err)
	}

	spansMu.Lock()
	if len(activeSpans) != 0 {
		t.Errorf("expected 0 active spans after end, got %d", len(activeSpans))
	}
	spansMu.Unlock()
}

func TestEndWithoutStart(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	tests := []struct {
		name      string
		endEvent  map[string]interface{}
		handler   func(map[string]interface{}) error
		wantError string
	}{
		{
			name: "http end without start",
			endEvent: map[string]interface{}{
				"event":    "http_request_end",
				"method":   "GET",
				"path":     "/api",
				"status":   float64(200),
				"duration": float64(1000000),
			},
			handler:   handleHTTPRequestEnd,
			wantError: "no active span found for HTTP",
		},
		{
			name: "dial end without start",
			endEvent: map[string]interface{}{
				"event":    "net_dial_end",
				"network":  "tcp",
				"address":  "localhost:5432",
				"duration": float64(1000000),
				"error":    float64(0),
			},
			handler:   handleNetDialEnd,
			wantError: "no active span found for dial",
		},
		{
			name: "tls end without start",
			endEvent: map[string]interface{}{
				"event":       "tls_handshake_end",
				"server_name": "example.com",
				"duration":    float64(1000000),
				"error":       float64(0),
			},
			handler:   handleTLSHandshakeEnd,
			wantError: "no active span found for TLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spansMu.Lock()
			activeSpans = make(map[string]*SpanContext)
			spansMu.Unlock()

			err := tt.handler(tt.endEvent)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestConcurrentSpans(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()

	// Start multiple HTTP requests concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startEvent := map[string]interface{}{
				"event":     "http_request_start",
				"method":    "GET",
				"path":      "/api/test",
				"timestamp": float64(1700000000000000000 + int64(idx)*1000000),
			}
			_ = handleHTTPRequestStart(ctx, startEvent)
		}(i)
	}
	wg.Wait()

	spansMu.Lock()
	startCount := len(activeSpans)
	spansMu.Unlock()

	if startCount != 10 {
		t.Errorf("expected 10 active spans, got %d", startCount)
	}
}

func TestSpanContextStorage(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	timestamp := int64(1700000000000000000)

	startEvent := map[string]interface{}{
		"event":     "http_request_start",
		"method":    "GET",
		"path":      "/test",
		"timestamp": float64(timestamp),
	}

	err := handleHTTPRequestStart(ctx, startEvent)
	if err != nil {
		t.Fatalf("handleHTTPRequestStart() error = %v", err)
	}

	spansMu.Lock()
	var spanCtx *SpanContext
	for _, v := range activeSpans {
		spanCtx = v
		break
	}
	spansMu.Unlock()

	if spanCtx == nil {
		t.Fatal("span context not stored")
	}

	expectedStartTime := time.Unix(0, timestamp)
	if !spanCtx.StartTime.Equal(expectedStartTime) {
		t.Errorf("StartTime = %v, want %v", spanCtx.StartTime, expectedStartTime)
	}

	if spanCtx.Span == nil {
		t.Error("Span is nil")
	}
}

// Mock tracer for testing span attributes
type mockSpan struct {
	noop.Span
	attributes []interface{}
	ended      bool
	endTime    time.Time
}

func (m *mockSpan) SetAttributes(kv ...interface{}) {
	m.attributes = append(m.attributes, kv...)
}

func (m *mockSpan) End(options ...oteltrace.SpanEndOption) {
	m.ended = true
	for _, opt := range options {
		if opt != nil {
			// Extract timestamp if provided
		}
	}
}
