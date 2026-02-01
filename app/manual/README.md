# Manual OpenTelemetry Instrumentation

This directory demonstrates traditional manual instrumentation using the OpenTelemetry Go SDK. This approach requires explicit code changes to add tracing, but provides full control over span creation, attributes, and context propagation.

## Overview

Manual instrumentation represents the conventional approach to adding observability to Go applications. Developers explicitly:

- Import OpenTelemetry SDK packages
- Initialize tracer providers and exporters
- Wrap HTTP handlers with middleware
- Create and manage spans in application code
- Configure span attributes and error handling

While this requires more work than automatic instrumentation, it offers complete control over the observability data generated.

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (manual)             │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ + OpenTelemetry SDK           │  │
│  │                               │  │
│  │ otelhttp.NewHandler()         │  │
│  │   ├─ Automatic HTTP spans     │  │
│  │   └─ Context propagation      │  │
│  │                               │  │
│  │ Manual span creation:         │  │
│  │   tracer.Start(ctx, "name")   │  │
│  │   defer span.End()            │  │
│  │                               │  │
│  │ OTLP HTTP Exporter            │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP (direct export)
           │
┌─────────────────────────────────────┐
│  OTel Collector                     │
└─────────────────────────────────────┘
```

No sidecar needed - the SDK exports traces directly from the application.

## Implementation Details

### Tracer Provider Setup

The application initializes OpenTelemetry during startup:

```go
func setupTracerProvider(inputs *Input) {
    ctx := context.Background()
    
    // Create OTLP HTTP exporter
    exporter, err := otlptracehttp.New(ctx,
        otlptracehttp.WithInsecure(),
        otlptracehttp.WithEndpoint(endpoint),
        otlptracehttp.WithTimeout(30*time.Second),
    )
    
    // Create resource with service metadata
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName("manual"),
            semconv.ServiceVersion("1.0.0"),
        ),
    )
    
    // Create tracer provider with batch processor
    provider = trace.NewTracerProvider(
        trace.WithBatcher(exporter,
            trace.WithBatchTimeout(5*time.Second),
            trace.WithMaxExportBatchSize(100),
            trace.WithMaxQueueSize(1000),
        ),
        trace.WithResource(res),
    )
    
    // Set global tracer provider
    otel.SetTracerProvider(provider)
}
```

### HTTP Middleware

Handlers are wrapped with `otelhttp.NewHandler()` for automatic HTTP instrumentation:

```go
func setupHandlers(inputs *Input) http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", HealthHandler)
    mux.HandleFunc("/load", inputs.LoadHandler)
    
    // Wrap with OpenTelemetry middleware
    return otelhttp.NewHandler(mux, "")
}
```

This middleware automatically:
- Creates spans for each HTTP request
- Extracts trace context from incoming headers
- Injects trace context into outgoing requests
- Adds semantic HTTP attributes

### Manual Span Creation

Handlers create custom spans for specific operations:

```go
func HealthHandler(w http.ResponseWriter, r *http.Request) {
    tracer := otel.Tracer("manual")
    _, span := tracer.Start(r.Context(), "manual.handler")
    defer span.End()
    
    io.WriteString(w, "OK\n")
}
```

## Dependencies

The manual approach requires these packages:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)
```

Binary size increases due to these dependencies.

## Configuration

### Environment Variables

```bash
# OTLP endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318

# Service name (also configurable in code)
OTEL_SERVICE_NAME=manual

# Resource attributes
OTEL_RESOURCE_ATTRIBUTES=environment=production,team=backend
```

### Code Configuration

The tracer provider configuration controls export behavior:

- **Batch Timeout**: How long to wait before forcing a batch export (5s)
- **Batch Size**: Maximum spans per batch (100)
- **Queue Size**: Maximum buffered spans before dropping (1000)
- **Retry**: Automatic retry with exponential backoff

## Usage

### Running with the test harness

```bash
go run . run --scenario manual --num 5
```

### Building manually

```bash
docker build -t manual --build-arg runtime_version=1.23 -f app/manual/Dockerfile .
```

### Running standalone

```bash
docker run -p 8080:8080 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318 \
  manual /app/inputs.json
```

## Comparison with Other Approaches

| Aspect | Manual | eBPF Auto | USDT | Orchestrion |
|--------|--------|-----------|------|-------------|
| Code changes | Heavy | None | None | None |
| Dependencies | OpenTelemetry SDK | None | None | OpenTelemetry SDK |
| Build complexity | Standard | Standard | CGO required | Modified build |
| Flexibility | Full control | Limited | Limited | High |
| Span customization | Complete | None | Custom probes | Aspect config |
| Context propagation | Automatic (middleware) | Automatic | Manual | Automatic |
| Learning curve | Medium | Low | High | Medium |
| Maintenance | High (code changes) | Low | Medium | Low |

### Advantages

1. **Full Control**: Precisely define what to instrument and how
2. **Rich Context**: Add custom attributes, events, and links to spans
3. **Type Safety**: Compile-time checking of instrumentation code
4. **Stable API**: OpenTelemetry SDK is production-ready and widely used
5. **Framework Integration**: Works with any framework or library
6. **Error Handling**: Explicit error reporting and span status management
7. **No External Dependencies**: No sidecars or privileged containers needed

### Limitations

1. **Code Maintenance**: Every instrumented operation requires explicit code
2. **Refactoring Overhead**: Changes to application structure require instrumentation updates
3. **Binary Size**: OpenTelemetry SDK increases binary size significantly
4. **Performance Overhead**: SDK operations add CPU and memory overhead
5. **Human Error**: Easy to forget instrumentation in new code paths
6. **Boilerplate**: Repetitive patterns for similar operations

## Best Practices

### 1. Centralize Tracer Creation

```go
var tracer = otel.Tracer("my-service")

func myHandler() {
    ctx, span := tracer.Start(ctx, "operation")
    defer span.End()
}
```

### 2. Always Use defer for span.End()

Ensures spans are closed even if the function panics.

### 3. Propagate Context

```go
func outer(ctx context.Context) {
    ctx, span := tracer.Start(ctx, "outer")
    defer span.End()
    
    inner(ctx) // Pass context to propagate trace
}
```

### 4. Add Meaningful Attributes

```go
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.Int("items.count", len(items)),
)
```

### 5. Record Errors

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    return err
}
```

## Files

- `main.go` - Application with manual OpenTelemetry instrumentation
- `Dockerfile` - Standard build with OTel dependencies
- `README.md` - This documentation

## When to Use Manual Instrumentation

Manual instrumentation is appropriate when:

- You need fine-grained control over span attributes and naming
- Custom business logic requires domain-specific tracing
- The application uses non-standard libraries without auto-instrumentation
- You want to add application-specific context to traces
- Performance profiling requires precise instrumentation points
- The team has expertise in OpenTelemetry and can maintain the code

## References

- [OpenTelemetry Go SDK](https://github.com/open-telemetry/opentelemetry-go)
- [OpenTelemetry Go Instrumentation Libraries](https://github.com/open-telemetry/opentelemetry-go-contrib)
- [OpenTelemetry API Documentation](https://opentelemetry.io/docs/languages/go/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
