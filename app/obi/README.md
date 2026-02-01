# OpenTelemetry eBPF Instrumentation (OBI)

This directory demonstrates automatic Go instrumentation using OpenTelemetry eBPF Instrumentation (OBI), a multi-language zero-code observability solution.

## Overview

OpenTelemetry eBPF Instrumentation (OBI) is a cross-language automatic instrumentation tool that uses eBPF to capture distributed traces from applications without code changes. Unlike the Go-specific auto-instrumentation, OBI supports multiple languages including Java, .NET, Go, Python, Ruby, Node.js, C, C++, and Rust.

OBI focuses on network-level instrumentation, capturing HTTP/S, gRPC, and gRPC-Web traffic by inspecting application executables and network calls using kernel-level tracing.

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (obi)                │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ (unmodified binary)           │  │
│  │                               │  │
│  │ - net/http server             │  │
│  │ - No code changes             │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ eBPF network inspection
           │
┌─────────────────────────────────────┐
│  OBI Agent                          │
│  ┌───────────────────────────────┐  │
│  │ eBPF Network Instrumentation  │  │
│  │ - HTTP/S traffic capture      │  │
│  │ - gRPC message tracing        │  │
│  │ - TLS/SSL visibility          │  │
│  │ - Distributed trace context   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/gRPC
           │
┌─────────────────────────────────────┐
│  OTel Collector                     │
└─────────────────────────────────────┘
```

## Key Features

OBI provides comprehensive network-level instrumentation:

- **Multi-Language Support**: Single agent instruments Java, .NET, Go, Python, Ruby, Node.js, C, C++, and Rust
- **Zero Code Changes**: Works with unmodified binaries
- **Protocol Coverage**:
    - HTTP and HTTPS (including encrypted traffic)
    - gRPC and gRPC-Web
    - Web transactions with RED metrics (Rate, Errors, Duration)
- **TLS/SSL Visibility**: Captures encrypted communications without decryption
- **Distributed Tracing**: Automatic trace context propagation across service boundaries
- **Kubernetes Native**: Configuration-free auto-instrumentation for K8s applications

## Supported Platforms

- **Go Versions**: 1.17+ (3 versions behind current stable maximum)
- **Operating System**: Linux only
- **Kernel Requirements**: Linux kernel 5.8+ (4.18+ for RHEL-based distributions)
- **Architectures**: x86_64, arm64
- **Container Runtime**: Any OCI-compliant runtime

## Usage

### Running with the test harness

```bash
go run . run --scenario obi --num 5
```

### Building manually

```bash
docker build -t obi --build-arg runtime_version=1.23 -f app/obi/Dockerfile .
```

### Deploying with Kubernetes

OBI is designed for Kubernetes environments with automatic discovery:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    instrumentation.opentelemetry.io/inject-sdk: "true"
spec:
  containers:
  - name: app
    image: my-go-app:latest
```

The OpenTelemetry Operator automatically injects the OBI agent.

## How It Works

OBI operates at the network layer:

1. **Process Discovery**: Identifies target processes in the system
2. **Network Hooking**: Attaches eBPF probes to network syscalls
3. **Traffic Capture**: Intercepts HTTP/gRPC requests and responses
4. **Protocol Parsing**: Analyzes application-layer protocols
5. **Span Generation**: Creates OpenTelemetry spans from network events
6. **Context Extraction**: Reads W3C Trace Context headers from traffic
7. **Export**: Sends spans to OTLP endpoint

Unlike language-specific instrumentation, OBI doesn't hook into application functions - it inspects network traffic at the kernel level.

## Comparison with Other Approaches

| Aspect | OBI | eBPF Auto (Go) | Manual | USDT |
|--------|-----|----------------|--------|------|
| Code changes | None | None | Heavy | None |
| Language support | Multi-language | Go only | Per-language | Any with USDT |
| Instrumentation level | Network | Application functions | Application code | Custom probes |
| Protocol coverage | HTTP/gRPC/TLS | HTTP/SQL/gRPC/Kafka | Unlimited | Custom |
| TLS inspection | Yes | No | No | No |
| Kubernetes native | Yes | Partial | No | No |
| Runtime overhead | Very low | Very low | Medium | Very low |
| Privileges required | Yes | Yes | No | Yes |
| Kernel version | 5.8+ | 5.8+ | None | 4.14+ |

### Advantages

1. **Language Agnostic**: Single solution for polyglot microservices
2. **Network Visibility**: Captures traffic regardless of application language
3. **TLS Transparency**: Sees encrypted traffic without certificate management
4. **Zero Maintenance**: No code changes or library updates required
5. **Kubernetes First**: Designed for cloud-native environments
6. **Consistent Instrumentation**: Same metrics across all services
7. **Retroactive**: Can instrument legacy applications without source code

### Limitations

1. **Network-Level Only**: Cannot trace in-process operations or database queries
2. **Limited Context**: No access to application-level business logic
3. **Protocol Specific**: Only understands HTTP, gRPC, and similar protocols
4. **Linux Only**: Requires Linux kernel with eBPF support
5. **Privileged Access**: Needs elevated permissions
6. **External Dependencies**: Requires OBI agent deployment
7. **Debugging Difficulty**: Lower-level traces may be harder to correlate with code

## Configuration

OBI is typically configured through the OpenTelemetry Operator:

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: obi-instrumentation
spec:
  ebpf:
    image: otel/ebpf-instrumentation:latest
  exporter:
    endpoint: http://otel-collector:4317
  resource:
    addK8sUIDAttributes: true
```

### Environment Variables

```bash
# OTLP endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317

# Service name
OTEL_SERVICE_NAME=obi-demo

# Resource attributes
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production
```

## Integration with Go Auto SDK

For Go applications, OBI can be combined with the Auto SDK to add manual spans:

```go
import "go.opentelemetry.io/auto"

func handler(w http.ResponseWriter, r *http.Request) {
    // OBI already created HTTP span at network level
    
    // Add application-level span
    ctx, span := auto.Tracer("my-service").Start(r.Context(), "business-logic")
    defer span.End()
    
    // Custom logic
}
```

This creates two-level instrumentation:
- Network-level HTTP span (from OBI)
- Application-level business logic span (from Auto SDK)

## Captured Data

OBI automatically captures:

### HTTP Spans

- Request method, URL, headers
- Response status code, headers
- Request/response duration
- Client and server IP addresses
- User agent strings

### gRPC Spans

- Service and method names
- Message sizes
- Status codes
- Metadata

### TLS Spans

- TLS version
- Cipher suite
- Server name (SNI)
- Certificate information

## Files

- `Dockerfile` - Builds application with standard Go toolchain
- `README.md` - This documentation

The OBI agent runs as a separate DaemonSet or sidecar container managed by the OpenTelemetry Operator.

## When to Use OBI

OBI is ideal when:

- Operating in Kubernetes environments
- Managing polyglot microservices (multiple languages)
- Instrumenting legacy applications without source access
- Needing network-level visibility across all services
- Capturing encrypted traffic patterns
- Standardizing observability across heterogeneous systems
- Prioritizing operational simplicity over deep code-level tracing

## Troubleshooting

### No Spans Generated

Check that:
- OBI agent has sufficient privileges (CAP_SYS_ADMIN or equivalent)
- Kernel version is 5.8+ (4.18+ for RHEL)
- Target process is using supported protocols (HTTP/gRPC)
- OTLP endpoint is reachable from the agent

### Missing Application Context

OBI only sees network traffic. For application-level context:
- Use the Auto SDK for manual spans
- Consider combining with language-specific instrumentation
- Add custom headers to HTTP requests

## References

- [OpenTelemetry eBPF Instrumentation](https://opentelemetry.io/docs/zero-code/obi/)
- [OBI Setup Guide](https://opentelemetry.io/docs/zero-code/obi/setup)
- [OBI Configuration](https://opentelemetry.io/docs/zero-code/obi/configure/)
- [OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator)
- [eBPF Documentation](https://ebpf.io/what-is-ebpf/)
