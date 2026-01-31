package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func setupTestTracer(t *testing.T) func() {
	t.Helper()

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer = otel.Tracer("test-bpftrace-exporter")

	activeSpans = make(map[string]*SpanContext)

	return func() {
		_ = tp.Shutdown(context.Background())
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
			name:      "request_start creates span",
			jsonLine:  `{"event":"request_start","reqid":"req-001","timestamp":1700000000000000000}`,
			wantErr:   false,
			checkSpan: true,
		},
		{
			name:     "missing event type",
			jsonLine: `{"reqid":"req-001","timestamp":1700000000000000000}`,
			wantErr:  true,
		},
		{
			name:     "missing reqid",
			jsonLine: `{"event":"request_start","timestamp":1700000000000000000}`,
			wantErr:  true,
		},
		{
			name:     "unknown event type",
			jsonLine: `{"event":"unknown_event","reqid":"req-001"}`,
			wantErr:  true,
		},
		{
			name:     "invalid json",
			jsonLine: `{invalid json}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activeSpans = make(map[string]*SpanContext)

			err := processEvent(context.Background(), tt.jsonLine)
			if (err != nil) != tt.wantErr {
				t.Errorf("processEvent() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkSpan {
				if len(activeSpans) == 0 {
					t.Error("expected span to be created, but activeSpans is empty")
				}
			}
		})
	}
}

func TestRequestLifecycle(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	reqID := "test-req-001"
	timestamp := float64(1700000000000000000)

	startEvent := map[string]any{
		"event":     "request_start",
		"reqid":     reqID,
		"timestamp": timestamp,
	}

	err := handleRequestStart(ctx, reqID, startEvent)
	if err != nil {
		t.Fatalf("handleRequestStart() error = %v", err)
	}

	if len(activeSpans) != 1 {
		t.Errorf("expected 1 active span, got %d", len(activeSpans))
	}

	if _, ok := activeSpans[reqID]; !ok {
		t.Errorf("span not stored with expected key %q", reqID)
	}

	endEvent := map[string]any{
		"event":    "request_end",
		"reqid":    reqID,
		"start":    timestamp,
		"duration": float64(5000000),
	}

	err = handleRequestEnd(reqID, endEvent)
	if err != nil {
		t.Fatalf("handleRequestEnd() error = %v", err)
	}

	if len(activeSpans) != 0 {
		t.Errorf("expected 0 active spans after end, got %d", len(activeSpans))
	}
}

func TestRequestStart_MissingTimestamp(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	reqID := "test-req-002"

	startEvent := map[string]any{
		"event": "request_start",
		"reqid": reqID,
	}

	err := handleRequestStart(ctx, reqID, startEvent)
	if err == nil {
		t.Error("expected error for missing timestamp, got nil")
	}
}

func TestRequestEnd_MissingDuration(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	reqID := "test-req-003"

	startEvent := map[string]any{
		"event":     "request_start",
		"reqid":     reqID,
		"timestamp": float64(1700000000000000000),
	}

	err := handleRequestStart(ctx, reqID, startEvent)
	if err != nil {
		t.Fatalf("handleRequestStart() error = %v", err)
	}

	endEvent := map[string]any{
		"event": "request_end",
		"reqid": reqID,
		"start": float64(1700000000000000000),
	}

	err = handleRequestEnd(reqID, endEvent)
	if err == nil {
		t.Error("expected error for missing duration, got nil")
	}
}

func TestRequestEnd_NoMatchingStart(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	reqID := "nonexistent-req"

	endEvent := map[string]any{
		"event":    "request_end",
		"reqid":    reqID,
		"start":    float64(1700000000000000000),
		"duration": float64(5000000),
	}

	err := handleRequestEnd(reqID, endEvent)
	if err == nil {
		t.Error("expected error for nonexistent request, got nil")
	}
}

func TestMultipleOverlappingRequests(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()

	requests := []struct {
		reqID     string
		timestamp float64
		duration  float64
	}{
		{"req-001", 1700000000000000000, 5000000},
		{"req-002", 1700000001000000000, 3000000},
		{"req-003", 1700000001500000000, 1000000},
	}

	for _, req := range requests {
		startEvent := map[string]any{
			"event":     "request_start",
			"reqid":     req.reqID,
			"timestamp": req.timestamp,
		}
		err := handleRequestStart(ctx, req.reqID, startEvent)
		if err != nil {
			t.Fatalf("handleRequestStart(%s) error = %v", req.reqID, err)
		}
	}

	if len(activeSpans) != 3 {
		t.Errorf("expected 3 active spans, got %d", len(activeSpans))
	}

	for _, req := range requests {
		endEvent := map[string]any{
			"event":    "request_end",
			"reqid":    req.reqID,
			"start":    req.timestamp,
			"duration": req.duration,
		}
		err := handleRequestEnd(req.reqID, endEvent)
		if err != nil {
			t.Fatalf("handleRequestEnd(%s) error = %v", req.reqID, err)
		}
	}

	if len(activeSpans) != 0 {
		t.Errorf("expected 0 active spans after all ends, got %d", len(activeSpans))
	}
}

func TestSpanContextStorage(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	reqID := "test-req-storage"
	timestamp := float64(1700000000123456789)

	startEvent := map[string]any{
		"event":     "request_start",
		"reqid":     reqID,
		"timestamp": timestamp,
	}

	err := handleRequestStart(ctx, reqID, startEvent)
	if err != nil {
		t.Fatalf("handleRequestStart() error = %v", err)
	}

	spanCtx, ok := activeSpans[reqID]
	if !ok {
		t.Fatal("span context not stored")
	}

	expectedStartTime := time.Unix(0, int64(timestamp))
	if !spanCtx.StartTime.Equal(expectedStartTime) {
		t.Errorf("StartTime = %v, want %v", spanCtx.StartTime, expectedStartTime)
	}

	if spanCtx.Span == nil {
		t.Error("Span is nil")
	}
}

// TestConcurrentRequestHandling tests that concurrent access to activeSpans is safe.
// NOTE: This test may expose race conditions if mutex protection is missing.
// Run with: go test -race ./app/libstabst/exporter/...
func TestConcurrentRequestHandling(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reqID := "concurrent-req-" + string(rune('A'+idx%26))
			timestamp := float64(1700000000000000000 + int64(idx)*1000000)

			startEvent := map[string]any{
				"event":     "request_start",
				"reqid":     reqID,
				"timestamp": timestamp,
			}
			_ = handleRequestStart(ctx, reqID, startEvent)

			endEvent := map[string]any{
				"event":    "request_end",
				"reqid":    reqID,
				"start":    timestamp,
				"duration": float64(1000000),
			}
			_ = handleRequestEnd(reqID, endEvent)
		}(i)
	}
	wg.Wait()
}

func TestProcessEventFromTestData(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()

	testEvents := []string{
		`{"event":"request_start","reqid":"req-001","timestamp":1769443100000000000}`,
		`{"event":"request_end","reqid":"req-001","start":1769443100000000000,"duration":2500000}`,
		`{"event":"request_start","reqid":"req-002","timestamp":1769443101000000000}`,
		`{"event":"request_start","reqid":"req-003","timestamp":1769443101500000000}`,
		`{"event":"request_end","reqid":"req-002","start":1769443101000000000,"duration":5000000}`,
		`{"event":"request_end","reqid":"req-003","start":1769443101500000000,"duration":1000000}`,
		`{"event":"request_start","reqid":"req-004","timestamp":1769443102000000000}`,
		`{"event":"request_end","reqid":"req-004","start":1769443102000000000,"duration":10000000}`,
	}

	for i, event := range testEvents {
		err := processEvent(ctx, event)
		if err != nil {
			t.Errorf("processEvent() for event %d error = %v", i, err)
		}
	}

	if len(activeSpans) != 0 {
		t.Errorf("expected 0 active spans after processing all events, got %d", len(activeSpans))
	}
}
