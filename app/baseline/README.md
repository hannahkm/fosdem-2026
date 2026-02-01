# Baseline (No Instrumentation)

This directory contains the baseline application with no instrumentation - serving as a performance control group for comparing the overhead of different instrumentation approaches.

## Overview

The baseline scenario runs the shared `app/main.go` without any observability instrumentation. It provides a clean baseline for measuring:

- Request latency without tracing overhead
- Memory usage without instrumentation libraries
- CPU utilization for uninstrumented code
- Binary size without added dependencies

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (baseline)           │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ (no instrumentation)          │  │
│  │                               │  │
│  │ - HTTP server (net/http)      │  │
│  │ - No OTel SDK                 │  │
│  │ - No tracing                  │  │
│  │ - No metrics                  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

No external dependencies for observability - just a pure HTTP server.

## Implementation Details

The baseline uses the shared application from `app/main.go`:

- **HTTP Endpoints**:
    - `/health` - Simple health check returning "OK"
    - `/load` - Simulates workload with CPU loops, memory allocations, and sleep

- **No Dependencies**: Zero observability libraries or frameworks
- **Standard Library Only**: Uses only `net/http`, `encoding/json`, and runtime packages
- **Minimal Overhead**: Represents the absolute minimum resource usage

## Usage

### Running with the test harness

```bash
go run . run --scenario baseline --num 5
```

### Building manually

```bash
docker build -t baseline --build-arg runtime_version=1.23 -f app/baseline/Dockerfile .
```

### Running standalone

```bash
docker run -p 8080:8080 baseline /app/inputs.json
```

## Comparison with Other Approaches

| Aspect | Baseline | Manual OTel | eBPF/USDT | Orchestrion |
|--------|----------|-------------|-----------|-------------|
| Code changes | None | Heavy | None | None |
| Dependencies | None | OpenTelemetry SDK | None | OpenTelemetry SDK |
| Overhead | Zero | Medium | Very low | Low |
| Tracing | None | Full | Automatic | Automatic |
| Binary size | Smallest | Larger | Same as baseline | Larger |
| Use case | Performance baseline | Full control needed | No code changes | Automated |

### Why Use Baseline?

1. **Performance Comparison**: Measure the true overhead of instrumentation approaches
2. **Regression Testing**: Ensure instrumentation doesn't degrade performance beyond acceptable limits
3. **Resource Planning**: Understand base resource requirements before adding observability
4. **Debugging**: Isolate whether issues are from application logic or instrumentation

## Files

- `Dockerfile` - Simple build from standard Go image
- `README.md` - This documentation

## Configuration

The baseline application reads configuration from a JSON file passed as the first argument:

```json
{
  "port": 8080,
  "off_cpu": 0.1,
  "loops_num": 1000,
  "allocs_num": 100,
  "alloc_size": 1024,
  "workers": 4
}
```

No observability-specific configuration needed.

## Limitations

1. No distributed tracing - cannot track requests across services
2. No metrics collection - manual monitoring required
3. No automatic error tracking
4. No performance profiling without external tools
5. Limited production observability

These limitations are intentional - the baseline demonstrates what you lose without instrumentation.

## References

- [Go net/http Package](https://pkg.go.dev/net/http)
- [Go Runtime Package](https://pkg.go.dev/runtime)
