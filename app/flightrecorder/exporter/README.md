# FlightRecorder OTLP Exporter

Sidecar service that watches for Go FlightRecorder trace files, converts trace events to OTLP spans, and exports them to the OpenTelemetry collector.

## Overview

This exporter bridges the gap between Go's runtime trace format and OpenTelemetry distributed tracing by:

1. Watching a shared volume for `.trace` files
2. Parsing FlightRecorder binary format using `golang.org/x/exp/trace`
3. Converting trace events to OTLP spans
4. Exporting spans via OTLP HTTP to the collector

## Architecture

```
┌─────────────────────────────────────┐
│  FlightRecorder App Container       │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ runtime/trace enabled         │  │
│  │                               │  │
│  │ Writes: /tmp/traces/*.trace   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ Shared Volume
           ▼
┌─────────────────────────────────────┐
│  Exporter Sidecar                   │
│  ┌───────────────────────────────┐  │
│  │ fsnotify File Watcher         │  │
│  │         ↓                     │  │
│  │ golang.org/x/exp/trace Parser │  │
│  │         ↓                     │  │
│  │ Event → Span Converter        │  │
│  │         ↓                     │  │
│  │ OTLP HTTP Exporter            │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP
           ▼
    OTel Collector → Jaeger
```

## Event Conversion

The exporter converts FlightRecorder events to OTLP spans, inspired by the bidirectional integration in [PR #7628](https://github.com/open-telemetry/opentelemetry-go/pull/7628) which adds `runtime/trace` Task and Region support to the OpenTelemetry Go SDK.

### Supported Event Types

| FlightRecorder Event | OTLP Representation | Description |
|---------------------|---------------------|-------------|
| `EventTaskBegin` | Start root span | `runtime/trace.NewTask` - similar to distributed trace spans |
| `EventTaskEnd` | End root span | Task completion with duration |
| `EventRegionBegin` | Start child span | `runtime/trace.StartRegion` - synchronous span-like regions |
| `EventRegionEnd` | End child span | Region completion with duration |
| `EventRangeBegin` | Start internal span | Generic ranges (GC, network I/O, etc.) |
| `EventRangeEnd` | End internal span | Range completion with duration |
| `EventLog` | Span event | `runtime/trace.Log` calls attached to spans |
| `EventStateTransition` | Span event | Goroutine/Proc state changes (running, waiting, etc.) |
| Stack frames | Span attributes | Function names and file locations |

### Task vs Region vs Range

The Go `runtime/trace` package provides three levels of instrumentation:

1. **Tasks** (`runtime/trace.NewTask`): Top-level operations that can span multiple goroutines. Maps to root spans in OTLP.

2. **Regions** (`runtime/trace.StartRegion`): Synchronous code sections within a task. Must begin and end on the same goroutine. Maps to child spans.

3. **Ranges**: Internal runtime instrumentation for GC, network I/O, etc. Maps to internal spans.

This distinction aligns with PR #7628's `ProfilingMode` configuration:

- `ProfilingDefault`: Root local spans create Tasks
- `ProfilingAuto`: Tracer decides Task vs Region based on span characteristics
- `ProfilingManual`: User explicitly controls Task/Region creation

## Configuration

Environment variables:

- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP collector endpoint (default: `otel-collector:4318`)
- `TRACE_OUTPUT_DIR` - Directory to watch for trace files (default: `/tmp/traces`)

## Usage

### With Test Harness

```bash
go run . run --scenario flightrecorder --num 5
```

The test harness automatically:

1. Builds the exporter image
2. Creates a shared volume
3. Starts both app and exporter containers
4. Connects everything to the network

### Manual Docker Compose

```bash
docker-compose up flightrecorder-exporter
```

Requires the `flightrecorder_traces` volume to be shared with the app container.

### Building Standalone

```bash
docker build -t flightrecorder-exporter -f Dockerfile .
docker run \
  -v flightrecorder_traces:/tmp/traces \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318 \
  flightrecorder-exporter
```

## Implementation Details

### File Watching

Uses `fsnotify` to detect new `.trace` files. Waits 500ms after file creation to ensure the file is fully written before processing.

### Trace Parsing

- Uses `golang.org/x/exp/trace.NewReader()` for binary format parsing
- Maintains separate maps for active spans and task spans (for parent-child relationships)
- Converts timestamps from trace time to wall clock time using first event as reference

### Span Relationships

- **Tasks** can be parents of other Tasks (via `task.Parent` field)
- **Regions** are always children of Tasks (via `region.Task` field)
- **Ranges** are standalone but associated with goroutines
- Log events are attached to their associated Task's span

### Span Attributes

Each span includes:

- `goroutine.id`: The goroutine that created the span
- `trace.event.kind`: "task", "region", or "range"
- `code.stacktrace.frames`: File:line locations (max 10)
- `code.stacktrace.functions`: Function names
- `duration_ns`/`duration_ms`: Span duration

### Error Handling

- Continues processing on individual file errors
- Logs warnings for unparseable files
- Marks unclosed spans with error status
- Graceful shutdown on SIGTERM/SIGINT

## Dependencies

- `github.com/fsnotify/fsnotify` - File system notifications
- `golang.org/x/exp/trace` - FlightRecorder binary format parser
- `go.opentelemetry.io/otel` - OpenTelemetry SDK
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` - OTLP HTTP exporter

## Limitations

- Does not replace `gotraceui` or other specialized trace analysis tools
- Conversion is lossy - some FlightRecorder details don't map to OTLP spans
- Stack traces limited to 10 frames for performance
- File watching assumes sequential writes (no partial file handling)
- No support for trace.FlightRecorder's circular buffer mode (only file output)

## Related Work

### opentelemetry-go PR #7628

The [PR #7628](https://github.com/open-telemetry/opentelemetry-go/pull/7628) adds config flags to control `runtime/trace` Task and Region creation from OTel spans:

```go
// Create a span that also creates a runtime/trace.Task
tracer.Start(ctx, "operation", trace.ProfileTask())

// Create a span that also creates a runtime/trace.Region
tracer.Start(ctx, "section", trace.ProfileRegion())

// Hint that span ends on different goroutine (affects Region creation)
tracer.Start(ctx, "async-op", trace.AsyncEnd())
```

This exporter provides the **reverse direction**: converting runtime/trace output back to OTLP spans for visualization in distributed tracing UIs like Jaeger.

### florianl/flightrecorderreceiver

| Aspect | florianl/flightrecorderreceiver | This Implementation |
|--------|--------------------------------|---------------------|
| Integration | Collector component | Standalone sidecar |
| Output | OTLP Profiles | OTLP Traces (spans) |
| Deployment | Requires custom collector build | Works with any collector |
| File handling | Glob pattern scraping | Active file watching |
| Event types | Focuses on profiling data | Task, Region, Range, Log, State |

## Files

- `main.go` - Entry point, file watcher, OTLP setup
- `convert.go` - Trace event to span conversion logic
- `Dockerfile` - Container image build
- `go.mod` - Go module dependencies
- `README.md` - This file

## References

- [opentelemetry-go PR #7628](https://github.com/open-telemetry/opentelemetry-go/pull/7628) - OTel + runtime/trace integration
- [florianl/flightrecorderreceiver](https://github.com/florianl/flightrecorderreceiver) - Profiles receiver implementation
- [golang.org/x/exp/trace](https://pkg.go.dev/golang.org/x/exp/trace) - Trace format documentation
- [Go Execution Tracer](https://go.dev/doc/diagnostics#tracer) - Official Go trace documentation
- [runtime/trace package](https://pkg.go.dev/runtime/trace) - Tasks, Regions, and Logs API
