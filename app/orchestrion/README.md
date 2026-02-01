# Orchestrion Compile-Time Instrumentation

This directory demonstrates automatic instrumentation using Datadog Orchestrion, a compile-time AST transformation tool that injects OpenTelemetry instrumentation without runtime code changes.

## Overview

Orchestrion is a vendor-neutral, compile-time instrumentation tool developed by Datadog that automatically adds observability code to Go applications during the build process. Unlike runtime approaches (eBPF) or manual instrumentation, Orchestrion uses Go's `-toolexec` mechanism to rewrite source code before compilation.

In January 2025, Datadog and Alibaba announced a collaboration to merge Orchestrion with Alibaba's opentelemetry-go-auto-instrumentation into a unified OpenTelemetry compile-time instrumentation solution.

## Architecture

```
┌─────────────────────────────────────┐
│  Build Time                         │
│  ┌───────────────────────────────┐  │
│  │ Source Code (app/main.go)     │  │
│  └───────────────┬───────────────┘  │
│                  │                   │
│                  ▼                   │
│  ┌───────────────────────────────┐  │
│  │ Orchestrion                   │  │
│  │ - AST analysis                │  │
│  │ - Join point matching         │  │
│  │ - Code injection              │  │
│  └───────────────┬───────────────┘  │
│                  │                   │
│                  ▼                   │
│  ┌───────────────────────────────┐  │
│  │ Instrumented Code             │  │
│  │ + OTel tracer setup           │  │
│  │ + HTTP middleware             │  │
│  │ + Span creation               │  │
│  └───────────────┬───────────────┘  │
│                  │                   │
│                  ▼                   │
│  ┌───────────────────────────────┐  │
│  │ Go Compiler                   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────┐
│  Runtime (Binary)                   │
│  ┌───────────────────────────────┐  │
│  │ Instrumented Application      │  │
│  │ - Auto-exports OTLP           │  │
│  │ - Distributed tracing         │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

## Key Features

Orchestrion provides compile-time automation with:

- **Zero Runtime Code Changes**: Original source remains unmodified
- **Aspect-Oriented Programming**: Declarative instrumentation via `orchestrion.yml`
- **Join Point System**: Match functions, packages, and code patterns
- **Advice Templates**: Inject code before, after, or around matched points
- **Go Toolchain Integration**: Works with standard `go build` via `-toolexec`
- **Vendor Neutral**: Supports OpenTelemetry (not Datadog-specific)
- **Standard Library Support**: Can instrument Go stdlib packages
- **Dependency Instrumentation**: Automatically instruments imported libraries

## Aspect Configuration

Orchestrion uses an aspect-oriented approach defined in `orchestrion.yml`:

### Join Points

Define where to inject code:

```yaml
join-point:
  all-of:
    - package-name: main
    - function-body:
        function:
          - name: processInputs
```

This matches the `processInputs` function in the `main` package.

### Advice

Define what code to inject:

```yaml
advice:
  - inject-declarations:
      imports:
        trace: go.opentelemetry.io/otel/sdk/trace
        otlptracehttp: go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
      template: |-
        func setupTracerProvider(inputs *Input) {
          // OTel initialization code
        }
  - prepend-statements:
      template: |-
        defer func() {
          setupTracerProvider({{ $inputs }})
        }()
```

Orchestrion generates and injects this code at compile time.

## Aspect Examples

This demo includes three aspects:

### 1. Initialize OTel Tracer

Injects tracer setup into the `processInputs` function:

- Creates OTLP exporter
- Configures tracer provider
- Sets resource attributes
- Registers global tracer

### 2. Shutdown OTel Tracer

Adds cleanup logic to `main()`:

- Deferred shutdown function
- Ensures spans are flushed
- Graceful resource cleanup

### 3. Instrument HTTP Handlers

Wraps HTTP handlers with OpenTelemetry:

- Injects `otelhttp.NewHandler()` middleware
- Creates spans for each request
- Propagates trace context
- Adds semantic HTTP attributes

## Usage

### Running with the test harness

```bash
go run . run --scenario orchestrion --num 5
```

### Building manually

```bash
# Install Orchestrion
go install github.com/DataDog/orchestrion@latest

# Build with instrumentation
cd app/orchestrion
orchestrion go build -o main .
```

### Dockerfile Build

The Dockerfile demonstrates the build process:

```dockerfile
# Install Orchestrion
RUN go install github.com/DataDog/orchestrion@latest

# Build with modified GOFLAGS
RUN GOFLAGS="-mod=mod" orchestrion go build -o main .
```

## How It Works

Orchestrion uses Go's build system hooks:

1. **Intercept Build**: The `orchestrion` wrapper intercepts `go build`
2. **AST Analysis**: Parses source files into Abstract Syntax Trees
3. **Pattern Matching**: Evaluates join points against AST nodes
4. **Code Generation**: Renders advice templates with matched context
5. **AST Transformation**: Injects generated code into the AST
6. **Compilation**: Passes modified code to the standard Go compiler
7. **Binary Output**: Produces instrumented binary

The original source files remain unchanged - transformation happens in-memory during build.

## Configuration

### orchestrion.yml

The configuration file defines all aspects:

```yaml
meta:
  name: otel-orchestrion
  description: Enables OpenTelemetry tracing with Orchestrion

aspects:
  - id: initialize-tracer
    join-point:
      # Where to inject
    advice:
      # What to inject
```

### Environment Variables

The generated code respects standard OpenTelemetry variables:

```bash
# OTLP endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318

# Service name
OTEL_SERVICE_NAME=orchestrion-demo

# Resource attributes
OTEL_RESOURCE_ATTRIBUTES=environment=production
```

## Comparison with Other Approaches

| Aspect | Orchestrion | Manual OTel | eBPF Auto | USDT |
|--------|-------------|-------------|-----------|------|
| Code changes | None (config only) | Heavy | None | None |
| Build process | Modified (toolexec) | Standard | Standard | Standard |
| Runtime overhead | Low (static) | Medium | Very low | Very low |
| Flexibility | High (aspects) | Complete | Limited | Medium |
| Customization | Config file | Code | None | Probe placement |
| Kernel requirements | None | None | 5.8+ | 4.14+ |
| Privileges | None | None | Required | Required |
| Debugging | Medium | Easy | Hard | Medium |
| Production ready | Yes | Yes | Beta | Yes |

### Advantages

1. **No Source Changes**: Instrumentation lives in configuration, not code
2. **Consistent Patterns**: Same instrumentation across all services
3. **Easy Updates**: Change config to update instrumentation
4. **Compile-Time Safety**: Errors caught during build, not runtime
5. **No Sidecar**: Instrumentation is part of the binary
6. **Dependency Injection**: Can instrument imported packages
7. **Vendor Neutral**: OpenTelemetry support (not locked to Datadog)
8. **Low Overhead**: Generated code is as efficient as manual code

### Limitations

1. **Build Complexity**: Requires orchestrion in build pipeline
2. **Learning Curve**: Aspect-oriented programming concepts
3. **YAML Configuration**: Complex aspects can become verbose
4. **Limited Inspection**: Can't see generated code without extra steps
5. **Go Specific**: Only works with Go (unlike OBI)
6. **Debugging**: Stack traces include generated code
7. **Binary Size**: Includes full OpenTelemetry SDK

## Advanced Aspects

### Conditional Injection

Use template conditionals to inject code selectively:

```yaml
template: |-
  {{ $inputs := .Function.ResultThatImplements "*Input" }}
  {{ if $inputs }}
    setupTracerProvider({{ $inputs }})
  {{- end -}}
```

### Multiple Join Points

Combine conditions with `all-of`, `any-of`, `not`:

```yaml
join-point:
  all-of:
    - package-name: main
    - function-body:
        function:
          - name: setupHandlers
```

### Import Management

Orchestrion automatically manages imports:

```yaml
imports:
  otelhttp: go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
  http: net/http
```

## Files

- `Dockerfile` - Multi-stage build with Orchestrion
- `orchestrion.yml` - Aspect configuration
- `README.md` - This documentation

The application source (`app/main.go`) is shared with the baseline scenario - no changes needed.

## Viewing Generated Code

To see what Orchestrion generates:

```bash
orchestrion go build -work -o main .
# Check the work directory for modified source files
```

Or enable debug mode:

```bash
ORCHESTRION_DEBUG=1 orchestrion go build -o main .
```

## Industry Collaboration

In January 2025, Datadog and Alibaba announced they would merge Orchestrion with Alibaba's opentelemetry-go-auto-instrumentation project. This collaboration aims to:

- Create a unified OpenTelemetry compile-time instrumentation standard
- Combine expertise from both organizations
- Provide vendor-neutral automatic instrumentation
- Support the broader Go and OpenTelemetry communities

This represents a significant shift toward standardized, automatic observability in Go.

## References

- [Orchestrion GitHub](https://github.com/DataDog/orchestrion)
- [Orchestrion Architecture](https://datadoghq.dev/orchestrion/docs/architecture/)
- [Datadog Blog: Orchestrion](https://datadoghq.com/blog/go-instrumentation-orchestrion)
- [OTel Blog: Go Compile-Time Instrumentation](https://opentelemetry.io/blog/2025/go-compile-time-instrumentation)
- [Orchestrion Troubleshooting](https://docs.datadoghq.com/tracing/troubleshooting/go_compile_time/)
