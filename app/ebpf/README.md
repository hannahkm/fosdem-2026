# OpenTelemetry Go Auto-Instrumentation (eBPF)

This directory demonstrates automatic Go instrumentation using the OpenTelemetry Go Auto-Instrumentation project with eBPF.

## Overview

OpenTelemetry Go Auto-Instrumentation enables zero-code distributed tracing by using eBPF (extended Berkeley Packet Filter) to instrument Go applications at runtime. The eBPF probes attach to running processes and capture trace data without requiring source code modifications or binary rebuilds.

This approach reached beta status in January 2025 and provides automatic instrumentation for common Go packages including `net/http`, `database/sql`, gRPC, and Kafka.

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (ebpf)               │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ (unmodified binary)           │  │
│  │                               │  │
│  │ - net/http server             │  │
│  │ - No code changes             │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ eBPF uprobes attach to process
           │
┌─────────────────────────────────────┐
│  OTel Auto-Instrumentation Agent    │
│  ┌───────────────────────────────┐  │
│  │ eBPF Instrumentation          │  │
│  │ - Captures function calls     │  │
│  │ - Propagates trace context    │  │
│  │ - Exports OTLP traces         │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP
           │
┌─────────────────────────────────────┐
│  OTel Collector                     │
└─────────────────────────────────────┘
```

## Key Features

The OpenTelemetry Go Auto-Instrumentation provides:

- **Zero Code Changes**: Instruments existing binaries without recompilation
- **Automatic Package Support**:
    - `net/http` - HTTP server and client instrumentation
    - `database/sql` - Database query tracing
    - `google.golang.org/grpc` - gRPC client and server
    - `github.com/segmentio/kafka-go` - Kafka message tracing
- **Context Propagation**: Automatic W3C Trace Context propagation across services
- **Semantic Conventions**: OpenTelemetry-compliant span attributes
- **Extensibility**: Can be combined with manual spans via the Auto SDK

## Supported Platforms

- **Go Versions**: 1.23, 1.24, 1.25+ (follows Go's upstream support policy)
- **Architectures**: amd64, arm64
- **Operating System**: Linux only
- **Kernel Requirements**: Linux kernel 5.8+ (4.18+ for RHEL-based distributions)

## Usage

### Running with the test harness

```bash
go run . run --scenario ebpf --num 5
```

### Building manually

```bash
docker build -t ebpf --build-arg runtime_version=1.23 -f app/ebpf/Dockerfile .
```

### Environment Variables

Configure the instrumentation agent using standard OpenTelemetry environment variables:

```bash
# OTLP endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318

# Service name
OTEL_SERVICE_NAME=ebpf-demo

# Enable specific instrumentations
OTEL_GO_AUTO_INSTRUMENTATION_ENABLED=true
```

## How It Works

The eBPF auto-instrumentation uses kernel uprobes to intercept function calls:

1. **Process Attachment**: The agent identifies the target Go process
2. **Symbol Resolution**: Locates instrumented functions in the binary using DWARF debug info
3. **eBPF Probe Installation**: Attaches uprobes to function entry/exit points
4. **Data Collection**: Captures arguments, return values, and timing
5. **Span Creation**: Converts captured data to OpenTelemetry spans
6. **Context Propagation**: Injects and extracts trace context from HTTP headers
7. **Export**: Sends spans to configured OTLP endpoint

## Comparison with Other Approaches

| Aspect | eBPF Auto | Manual OTel | USDT | Orchestrion |
|--------|-----------|-------------|------|-------------|
| Code changes | None | Heavy | None | None |
| Build process | Standard | Standard | Standard | Modified (toolexec) |
| Runtime overhead | Very low | Medium | Very low | Low |
| Sidecar needed | Yes (agent) | No | Yes (tracer) | No |
| Kernel requirements | 5.8+ | None | 4.14+ | None |
| Privileges | Required | None | Required | None |
| Customization | Limited | Full | Medium | High |
| Production ready | Beta | Yes | Yes | Yes |

### Advantages

1. **Zero Code Modification**: Instrument legacy applications without changing code
2. **Automatic Updates**: New instrumentation features work with existing binaries
3. **Consistent Instrumentation**: Standardized across all Go applications
4. **Low Overhead**: eBPF runs efficiently in kernel space
5. **No Redeployment**: Can enable/disable without rebuilding

### Limitations

1. **Linux Only**: Requires Linux kernel with eBPF support
2. **Privileged Access**: Needs CAP_SYS_ADMIN or similar capabilities
3. **Limited Packages**: Only supports specific libraries (expanding over time)
4. **Beta Status**: May have stability or compatibility issues
5. **Debug Symbols**: Works best with binaries built with debug information
6. **Agent Dependency**: Requires separate instrumentation agent process

## Files

- `Dockerfile` - Builds application with standard Go toolchain
- `README.md` - This documentation

The actual eBPF instrumentation agent runs as a separate container managed by the test harness.

## Integration with Manual Spans

The Auto SDK allows combining automatic eBPF instrumentation with manual spans:

```go
import "go.opentelemetry.io/auto"

func handler(w http.ResponseWriter, r *http.Request) {
    // Automatic eBPF span already created for this handler
    
    // Add custom span that correlates with eBPF traces
    ctx, span := auto.Tracer("my-service").Start(r.Context(), "custom-operation")
    defer span.End()
    
    // Your business logic
}
```

Since OpenTelemetry Go v1.36.0, the Auto SDK is automatically available as an indirect dependency.

## References

- [OpenTelemetry Go Auto-Instrumentation](https://github.com/open-telemetry/opentelemetry-go-instrumentation)
- [Beta Release Announcement (January 2025)](https://opentelemetry.io/blog/2025/go-auto-instrumentation-beta)
- [Go Auto SDK Documentation](https://opentelemetry.io/docs/zero-code/go/autosdk/)
- [eBPF Introduction](https://ebpf.io/what-is-ebpf/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
