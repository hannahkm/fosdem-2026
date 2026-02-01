# Flight Recorder Distributed Tracing Demo

This directory demonstrates always-on distributed tracing using Go's Flight Recorder with a custom fork that adds distributed tracing capabilities and W3C Trace Context correlation.

## Status: Proof of Concept

This is a **working PoC** demonstrating FlightRecorder trace conversion to OTLP spans. The implementation:

- Builds and runs successfully with Go 1.25
- Converts FlightRecorder events (Tasks, Regions, Ranges, Logs, State Transitions) to OTLP spans
- Exports traces to OpenTelemetry Collector via sidecar
- Visualizes traces in Jaeger

### Current Limitations

1. **Requires custom Go fork** - Not upstream, must build from source
2. **Sidecar architecture** - Uses file-based trace sharing between containers
3. **Timestamp approximation** - Wall clock times are approximated from trace timestamps
4. **Limited event coverage** - Not all FlightRecorder events are converted
5. **No circular buffer support** - Only processes complete trace files

### Remaining Work

- [ ] Upstream FlightRecorder improvements to Go
- [ ] Direct OTLP export from runtime (no sidecar)
- [ ] W3C Trace Context propagation between services
- [ ] Support for circular buffer mode
- [ ] Better timestamp synchronization using ClockSnapshot
- [ ] Integration with [opentelemetry-go PR #7628](https://github.com/open-telemetry/opentelemetry-go/pull/7628)

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
# Run a single test
go run . run --scenario flightrecorder

# Run multiple iterations
go run . run --scenario flightrecorder --num 5

# Run with custom timeout (default: 5m)
go run . run --scenario flightrecorder --timeout 3m

# Force rebuild without cache
go run . run --scenario flightrecorder --force
```

### What the test harness does

1. Builds the FlightRecorder Go fork from source
2. Builds the application with FlightRecorder-enabled runtime
3. Builds and starts the [OTLP exporter sidecar](./exporter/README.md)
4. Creates shared volume for trace files
5. Starts OTel Collector and Jaeger
6. Runs the application and exports traces
7. Cleans up containers after test

### Viewing traces

After running a test, view traces in Jaeger:

```bash
open http://localhost:16686
```

Look for traces with service name `flightrecorder` containing spans for Tasks, Regions, and Ranges.

### Building manually

```bash
# Build app container (includes Go fork compilation)
docker build -t flightrecorder -f app/flightrecorder/Dockerfile .

# Build exporter sidecar
docker build -t flightrecorder-exporter -f app/flightrecorder/exporter/Dockerfile app/flightrecorder/exporter/
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

## Testing

### End-to-end test

```bash
# From project root
go run . run --scenario flightrecorder --num 1 --timeout 3m
```

Expected output includes:
- `Scenario completed ... failures=0`
- Traces visible in Jaeger at http://localhost:16686

### Verifying trace conversion

Check exporter logs for conversion activity:

```bash
docker logs flightrecorder-exporter 2>&1 | grep -i "span\|event\|trace"
```

### Troubleshooting

1. **Build fails with Go bootstrap error**: Ensure Docker has enough memory (4GB+)
2. **No traces in Jaeger**: Check that exporter sidecar is running and connected to network
3. **Container conflicts**: Run `docker compose down --remove-orphans` before retrying

## Comparison with Other Approaches

| Aspect | FlightRecorder (vision) | This PoC | Manual OTel | eBPF/USDT |
|--------|------------------------|----------|-------------|-----------|
| Code changes | None | None | Extensive | None |
| Overhead | Very low | Low | Medium | Very low |
| Sidecar needed | No | Yes* | No | Yes |
| Distributed tracing | Built-in | Partial | Manual | Manual |
| Filtering | GODEBUG/API | Limited | Manual | Manual |
| Data safety | Auto-sanitized | N/A | Manual | Manual |
| Ring buffer | Yes | No | No | No |

*This PoC uses a sidecar for trace export. The fork's vision is direct OTLP export from runtime.

### Advantages (Full FlightRecorder Vision)

1. **Always-on**: Ring buffer design for minimal overhead
2. **No code changes**: Works via GODEBUG environment
3. **Direct export**: No sidecar containers needed (future)
4. **Distributed tracing**: W3C Trace Context built-in
5. **Data safety**: Automatic URL/SQL sanitization
6. **Bounded memory**: Ring buffer prevents unbounded growth

### Current PoC Advantages

1. **Working end-to-end**: Demonstrates trace conversion concept
2. **Standard tooling**: Works with any OTel Collector and Jaeger
3. **No runtime modifications**: Uses file-based trace export
4. **Extensible**: Easy to add new event type conversions

### Limitations

1. Requires custom Go fork (not upstream)
2. Linux-centric features (some may work on other platforms)
3. Build time significantly longer (compiles Go from source)
4. PoC uses sidecar (not direct export yet)

## Files

- `Dockerfile` - Multi-stage build from Go fork
- `README.md` - This documentation
- `exporter/` - OTLP exporter sidecar ([see exporter README](./exporter/README.md))
  - `main.go` - Entry point, file watcher, OTLP setup
  - `convert.go` - Trace event to span conversion logic
  - `Dockerfile` - Container image build
  - `go.mod` - Go module dependencies (Go 1.25, OTel v1.34.0)

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (flightrecorder)     │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ (built with FlightRecorder Go)│  │
│  │                               │  │
│  │ Writes: /tmp/traces/*.trace   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ Shared Volume (flightrecorder_traces)
           ▼
┌─────────────────────────────────────┐
│  Exporter Sidecar                   │
│  ┌───────────────────────────────┐  │
│  │ fsnotify → trace.Reader →     │  │
│  │ OTLP Spans → HTTP Export      │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP (:4318)
           ▼
┌─────────────────────────────────────┐
│  OTel Collector → Jaeger (:16686)  │
└─────────────────────────────────────┘
```

## Related Work

### opentelemetry-go PR #7628

The [PR #7628](https://github.com/open-telemetry/opentelemetry-go/pull/7628) adds bidirectional integration between OTel spans and `runtime/trace`:

- **Forward direction**: OTel spans create `runtime/trace` Tasks and Regions
- **Reverse direction** (this PoC): Convert `runtime/trace` output back to OTLP spans

Our exporter implements the reverse direction, enabling visualization of runtime traces in distributed tracing UIs.

### florianl/flightrecorderreceiver

The [flightrecorderreceiver](https://github.com/florianl/flightrecorderreceiver) takes a different approach:

| Aspect | flightrecorderreceiver | This PoC |
|--------|------------------------|----------|
| Integration | Collector component | Standalone sidecar |
| Output | OTLP Profiles | OTLP Traces (spans) |
| Deployment | Custom collector build | Works with any collector |
| Focus | Profiling data | Task, Region, Range, Log events |

## References

- [Go Fork: poc_flight_recorder](https://github.com/kakkoyun/go/tree/poc_flight_recorder)
- [opentelemetry-go PR #7628](https://github.com/open-telemetry/opentelemetry-go/pull/7628) - OTel + runtime/trace integration
- [florianl/flightrecorderreceiver](https://github.com/florianl/flightrecorderreceiver) - Profiles receiver implementation
- [golang.org/x/exp/trace](https://pkg.go.dev/golang.org/x/exp/trace) - Trace format documentation
- [Go Execution Tracer](https://go.dev/doc/diagnostics#tracer) - Official Go trace documentation
- [runtime/trace package](https://pkg.go.dev/runtime/trace) - Tasks, Regions, and Logs API
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
