package core

import (
	"fmt"
	"sync"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// SpanManager provides thread-safe management of active spans.
type SpanManager struct {
	mu     sync.Mutex
	spans  map[string]*SpanContext
	tracer oteltrace.Tracer
}

// NewSpanManager creates a new SpanManager with the given tracer.
func NewSpanManager(tracer oteltrace.Tracer) *SpanManager {
	return &SpanManager{
		spans:  make(map[string]*SpanContext),
		tracer: tracer,
	}
}

// Tracer returns the underlying tracer.
func (m *SpanManager) Tracer() oteltrace.Tracer {
	return m.tracer
}

// Store adds a span context to the manager.
func (m *SpanManager) Store(key string, ctx *SpanContext) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans[key] = ctx
}

// Get retrieves a span context by key without removing it.
func (m *SpanManager) Get(key string) (*SpanContext, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, ok := m.spans[key]
	return ctx, ok
}

// Remove retrieves and removes a span context by key.
func (m *SpanManager) Remove(key string) (*SpanContext, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, ok := m.spans[key]
	if ok {
		delete(m.spans, key)
	}
	return ctx, ok
}

// FindByPrefix finds and removes the first span with a key matching the given prefix.
func (m *SpanManager) FindByPrefix(prefix string) (*SpanContext, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, ctx := range m.spans {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(m.spans, key)
			return ctx, key, nil
		}
	}
	return nil, "", fmt.Errorf("no span found with prefix %q", prefix)
}

// Count returns the number of active spans.
func (m *SpanManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.spans)
}

// Clear removes all active spans.
func (m *SpanManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans = make(map[string]*SpanContext)
}
