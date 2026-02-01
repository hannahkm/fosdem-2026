# BPFTrace to OpenTelemetry Exporter

This directory contains a unified exporter that bridges bpftrace events to OpenTelemetry traces. It is used by the USDT-based instrumentation scenarios (`usdt/` and `libstabst/`) to convert low-level trace events into distributed traces.

## Overview

The exporter runs as a sidecar container that:

1. Executes bpftrace scripts against a target process
2. Parses JSON-formatted events from bpftrace output
3. Correlates start/end events to create complete spans
4. Exports spans to an OpenTelemetry collector via OTLP

This shared component eliminates duplication between different USDT approaches and provides a reusable bridge between kernel-level tracing and OpenTelemetry.

## Architecture

```
┌─────────────────────────────────────┐
│  App Container                      │
│  (usdt or libstabst)                │
│  ┌───────────────────────────────┐  │
│  │ Go Application with USDT      │  │
│  │ probes (runtime or compiled)  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ PID namespace sharing
           │
┌─────────────────────────────────────┐
│  Exporter Container (go-usdt)       │
│  ┌───────────────────────────────┐  │
│  │ bpftrace                      │  │
│  │ ├─ Attaches to USDT probes    │  │
│  │ └─ Outputs JSON events        │  │
│  │                               │  │
│  │ Exporter (this package)       │  │
│  │ ├─ Parses JSON events         │  │
│  │ ├─ Manages span lifecycle     │  │
│  │ └─ Exports OTLP traces        │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP
           │
┌─────────────────────────────────────┐
│  OTel Collector                     │
└─────────────────────────────────────┘
```

## Components

### Core Exporter (`core/exporter.go`)

Main orchestration component that:

- Starts bpftrace with JSON output format
- Reads events from stdout
- Dispatches events to registered handlers
- Manages OpenTelemetry tracer and shutdown

### Span Manager (`core/span_manager.go`)

Tracks active spans and correlates start/end events:

- Creates spans from start events
- Stores spans in memory by request ID
- Completes spans when end events arrive
- Adds attributes and status codes
- Handles span cleanup and timeout

### Event Handlers (`handlers/`)

Specialized handlers for different event types:

- **HTTP Handler** (`http.go`): HTTP request/response spans
- **TLS Handler** (`tls.go`): TLS handshake spans
- **Dial Handler** (`dial.go`): Network connection spans
- **Request Handler** (`request.go`): Generic request lifecycle

Each handler:
- Implements the `EventHandler` interface
- Parses event-specific data
- Extracts semantic attributes
- Creates properly named spans

## Event Format

BPFTrace scripts emit events in this JSON format:

```json
{
  "event": "http_request_start",
  "data": {
    "request_id": "abc123",
    "method": "GET",
    "uri": "/api/users",
    "timestamp": 1706745600000000000
  }
}
```

```json
{
  "event": "http_request_end",
  "data": {
    "request_id": "abc123",
    "status_code": 200,
    "duration": 15000000
  }
}
```

The exporter matches start/end events by `request_id` to create complete spans.

## Configuration

Configure via environment variables:

```bash
# Required: Target process PID to attach bpftrace
TARGET_PID=1

# Required: Path to bpftrace script
BPF_SCRIPT=/app/trace.bt

# Required: OpenTelemetry collector endpoint
OTEL_ENDPOINT=otel-collector:4318

# Optional: Service name for traces
SERVICE_NAME=usdt-demo

# Optional: Tracer name
TRACER_NAME=bpftrace-exporter

# Optional: Operating mode (usdt or native-usdt)
EXPORTER_MODE=usdt
```

## Usage

### As a Sidecar

The exporter is typically run as a sidecar container sharing the PID namespace with the instrumented application:

```yaml
services:
  app:
    image: usdt-app
    container_name: usdt-app
    
  exporter:
    image: go-usdt-exporter
    pid: "container:usdt-app"
    privileged: true
    environment:
      - TARGET_PID=1
      - BPF_SCRIPT=/app/trace.bt
      - OTEL_ENDPOINT=otel-collector:4318
      - SERVICE_NAME=my-service
```

### Building

```bash
docker build -t go-usdt-exporter -f app/exporter/Dockerfile .
```

### Testing

The exporter includes unit tests that verify event parsing without requiring bpftrace:

```bash
cd app/exporter
go test ./core/... -v
```

Test data is provided in `test-data.json` for validation.

## Supported Event Types

| Event Type | Handler | Span Name | Key Attributes |
|------------|---------|-----------|----------------|
| `http_request_start/end` | HTTP | `HTTP {METHOD} {URI}` | http.method, http.url, http.status_code |
| `tls_handshake_start/end` | TLS | `TLS Handshake` | tls.server_name, tls.version |
| `net_dial_start/end` | Dial | `net.Dial` | net.peer.name, net.peer.port |
| `request_start/end` | Request | `Request` | request.id, request.duration |

## Adding New Handlers

To support new event types:

1. Create a new handler in `handlers/`:

```go
type MyHandler struct {
    spanManager *core.SpanManager
}

func (h *MyHandler) CanHandle(eventType string) bool {
    return eventType == "my_event_start" || eventType == "my_event_end"
}

func (h *MyHandler) HandleStart(ctx context.Context, event core.Event) error {
    // Extract attributes
    requestID := event.GetString("request_id")
    
    // Create span
    span := h.spanManager.StartSpan(ctx, requestID, "My Operation")
    span.SetAttributes(/* ... */)
    
    return nil
}

func (h *MyHandler) HandleEnd(event core.Event) error {
    requestID := event.GetString("request_id")
    h.spanManager.EndSpan(requestID)
    return nil
}
```

2. Register the handler in `cmd/main.go`:

```go
exporter.RegisterHandler(&handlers.MyHandler{})
```

3. Update your bpftrace script to emit the events.

## Comparison with Direct Instrumentation

| Aspect | BPFTrace Exporter | Direct OTel SDK |
|--------|-------------------|-----------------|
| Application changes | None | Heavy |
| Language support | Any (via USDT) | Language-specific |
| Overhead | Very low (eBPF) | Medium |
| Span creation | Post-processed | Real-time |
| Context propagation | Manual extraction | Automatic |
| Flexibility | Limited to events | Full control |

## Files

- `cmd/main.go` - Entry point and CLI
- `core/exporter.go` - Main exporter logic
- `core/span_manager.go` - Span lifecycle management
- `core/config.go` - Configuration handling
- `core/types.go` - Event and handler interfaces
- `handlers/http.go` - HTTP event handler
- `handlers/tls.go` - TLS event handler
- `handlers/dial.go` - Network dial handler
- `handlers/request.go` - Generic request handler
- `Dockerfile` - Container build
- `test-unified.sh` - Integration test script
- `README.md` - This documentation

## Limitations

1. **Stateful Matching**: Requires memory to track active spans (bounded by application concurrency)
2. **Event Loss**: If bpftrace drops events, spans may be incomplete
3. **Time Skew**: Timestamps from kernel may differ from application time
4. **PID Sharing**: Requires privileged container or CAP_SYS_PTRACE
5. **Linux Only**: bpftrace requires Linux kernel 4.14+

## References

- [bpftrace Documentation](https://github.com/iovisor/bpftrace)
- [OpenTelemetry Trace API](https://opentelemetry.io/docs/specs/otel/trace/api/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
