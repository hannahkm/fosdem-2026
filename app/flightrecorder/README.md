# Flight Recorder Distributed Tracing Demo

This directory demonstrates always-on distributed tracing using Go's Flight Recorder with a custom fork that adds distributed tracing capabilities and W3C Trace Context correlation.

## Overview

Go's Flight Recorder provides ring-buffer-based tracing with minimal overhead. This fork extends it with:

- Distributed tracing correlation across services
- W3C Trace Context propagation
- Event filtering for selective tracing
- Direct OTLP export without sidecars

## Go Fork

Based on [kakkoyun/go:poc_flight_recorder](https://github.com/kakkoyun/go/tree/poc_flight_recorder)

### Features

- **Event Filtering API**: Bitmask system for selective tracing categories
- **W3C Trace Context**: Automatic propagation for distributed trace correlation
- **GODEBUG Configuration**: Environment-based trace enablement
- **Specialized Span Types**: HTTPSpan, SQLSpan, TLSSpan, DNSSpan, ConnectSpan
- **Data Sanitizers**: Built-in URL and SQL sanitization to prevent data leakage
- **Ring Buffer**: Bounded memory usage for always-on tracing

## Configuration

### Environment Variables (GODEBUG)

Enable tracing categories via GODEBUG environment variable:

```bash
# Enable specific categories
GODEBUG=tracehttp=1           # HTTP client/server tracing
GODEBUG=tracesql=1            # Database query tracing
GODEBUG=tracetls=1            # TLS handshake tracing
GODEBUG=tracenet=1            # Network operation tracing

# Enable multiple categories
GODEBUG=tracehttp=1,tracesql=1,tracenet=1

# Enable all categories
GODEBUG=traceall=1
```

### Programmatic API

For fine-grained control:

```go
import "runtime/trace/flight"

func main() {
    // Set filter categories
    flight.SetFilter(flight.FilterHTTP | flight.FilterSQL)

    // Enable distributed tracing correlation
    flight.EnableDistributedTracing(true)

    // Set OTLP endpoint
    flight.SetExportEndpoint("otel-collector:4318")

    // ... application code
}
```

### Filter Categories

| Category | Constant | Description |
|----------|----------|-------------|
| Core | `FilterCore` | Core runtime events (GC, scheduling) |
| HTTP | `FilterHTTP` | HTTP client and server operations |
| SQL | `FilterSQL` | Database queries and connections |
| TLS | `FilterTLS` | TLS handshakes and sessions |
| Net | `FilterNet` | Network dial, listen, accept |
| Custom | `FilterCustom` | Application-defined trace points |

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (flightrecorder)     │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ (built with FlightRecorder Go)│  │
│  │                               │  │
│  │ GODEBUG: tracehttp=1,tracenet │  │
│  │                               │  │
│  │ Auto-exports traces via OTLP  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP (direct export)
           │
┌─────────────────────────────────────┐
│  OTel Collector                     │
└─────────────────────────────────────┘
```

Unlike USDT approaches, Flight Recorder exports traces directly - no sidecar needed.

## Specialized Span Types

The fork provides structured span types with semantic attributes:

### HTTPSpan

```go
type HTTPSpan struct {
    Method     string
    URL        string // Sanitized
    StatusCode int
    Duration   time.Duration
    TraceID    string
    SpanID     string
}
```

### SQLSpan

```go
type SQLSpan struct {
    Query     string // Sanitized (no literals)
    Database  string
    Duration  time.Duration
    RowCount  int64
    Error     error
}
```

### TLSSpan

```go
type TLSSpan struct {
    ServerName    string
    Version       uint16
    CipherSuite   uint16
    Duration      time.Duration
    Resumed       bool
}
```

## Data Sanitization

Built-in sanitizers prevent sensitive data leakage:

- **URL Sanitizer**: Removes query parameters, credentials
- **SQL Sanitizer**: Replaces literals with placeholders

```go
// Original: SELECT * FROM users WHERE email = 'user@example.com'
// Sanitized: SELECT * FROM users WHERE email = ?
```

## Usage

### Running with the test harness

```bash
go run . run --scenario flightrecorder --num 5
```

### Building manually

```bash
docker build -t flightrecorder -f app/flightrecorder/Dockerfile .
```

### Running with different trace categories

```bash
# HTTP and SQL tracing
docker run -e GODEBUG=tracehttp=1,tracesql=1 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318 \
  flightrecorder /app/inputs.json

# All tracing enabled
docker run -e GODEBUG=traceall=1 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318 \
  flightrecorder /app/inputs.json
```

## Comparison with Other Approaches

| Aspect | FlightRecorder | Manual OTel | eBPF/USDT |
|--------|---------------|-------------|-----------|
| Code changes | None | Extensive | None |
| Overhead | Very low | Medium | Very low |
| Sidecar needed | No | No | Yes |
| Distributed tracing | Built-in | Manual | Manual |
| Filtering | GODEBUG/API | Manual | Manual |
| Data safety | Auto-sanitized | Manual | Manual |
| Ring buffer | Yes | No | No |

### Advantages

1. **Always-on**: Ring buffer design for minimal overhead
2. **No code changes**: Works via GODEBUG environment
3. **Direct export**: No sidecar containers needed
4. **Distributed tracing**: W3C Trace Context built-in
5. **Data safety**: Automatic URL/SQL sanitization
6. **Bounded memory**: Ring buffer prevents unbounded growth

### Limitations

1. Requires custom Go fork (not upstream)
2. Linux-centric features (some may work on other platforms)
3. Build time significantly longer (compiles Go from source)

## Files

- `Dockerfile` - Multi-stage build from Go fork
- `README.md` - This documentation

## References

- [Go Fork: poc_flight_recorder](https://github.com/kakkoyun/go/tree/poc_flight_recorder)
- [Go Execution Tracer](https://go.dev/doc/diagnostics#tracer)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
