# Native USDT Probes Demo

This directory demonstrates Go instrumentation using native USDT (User Statically-Defined Tracing) probes built into a custom Go fork.

## Overview

Unlike the `libstabst` variant which uses the `salp` library to create probes at runtime, this approach embeds USDT probes directly in the Go compiler and standard library. The probes are statically compiled into binaries and have near-zero overhead when not actively traced.

## Go Fork

Based on [kakkoyun/go:poc_usdt](https://github.com/kakkoyun/go/tree/poc_usdt)

### Features

- **Compiler/Linker Modifications**: USDTProbe SSA operation for AMD64 and ARM64
- **ELF Integration**: Emits `.note.stapsdt` sections with probe metadata
- **New Package**: `runtime/trace/usdt` with Probe, Probe1, Probe2, Probe3 functions
- **Stdlib Integration**: Automatic probes in net/http, database/sql, crypto/tls, net
- **Tooling**: `go tool usdt` command for listing and validating probes

### Supported Platforms

- linux/amd64
- linux/arm64

## Architecture

```
┌─────────────────────────────────────┐
│  App Container (usdt)               │
│  ┌───────────────────────────────┐  │
│  │ Go Application                │  │
│  │ (built with USDT-enabled Go)  │  │
│  │                               │  │
│  │ Stdlib auto-instrumented:     │  │
│  │ - net/http                    │  │
│  │ - database/sql                │  │
│  │ - crypto/tls                  │  │
│  │ - net                         │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ PID namespace sharing
           │
┌─────────────────────────────────────┐
│  Tracer Container (go-usdt)         │
│  ┌───────────────────────────────┐  │
│  │ bpftrace + OTel Exporter      │  │
│  │ (attaches to stdlib probes)   │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           │
           │ OTLP/HTTP
           │
┌─────────────────────────────────────┐
│  OTel Collector                     │
└─────────────────────────────────────┘
```

## Automatic Probe Points

The fork instruments these stdlib packages automatically:

| Package | Probe | Arguments |
|---------|-------|-----------|
| net/http | `go:http_server_request_start` | method, path, timestamp |
| net/http | `go:http_server_request_end` | method, path, status, duration |
| database/sql | `go:sql_query_start` | query, timestamp |
| database/sql | `go:sql_query_end` | query, duration, error |
| crypto/tls | `go:tls_handshake_start` | server_name, timestamp |
| crypto/tls | `go:tls_handshake_end` | server_name, duration, error |
| net | `go:net_dial_start` | network, address, timestamp |
| net | `go:net_dial_end` | network, address, duration, error |

## Usage

### Running with the test harness

```bash
go run . run --scenario usdt --num 5
```

### Building manually

```bash
docker build -t usdt -f app/usdt/Dockerfile .
```

### Listing probes in a binary

After building with the USDT-enabled Go:

```bash
# Inside a container with the custom Go
go tool usdt list ./main
```

Or using readelf:

```bash
readelf -n ./main | grep -A 4 stapsdt
```

### Attaching bpftrace manually

```bash
# Start the app container
docker run --name usdt-app -p 8080:8080 usdt /app/inputs.json

# In another terminal, attach bpftrace
docker run --privileged --pid=container:usdt-app \
  -v $(pwd)/app/usdt/exporter/trace-json.bt:/trace.bt:ro \
  quay.io/iovisor/bpftrace:latest bpftrace /trace.bt -p 1

# Generate load
curl http://localhost:8080/load
```

## Comparison with libstabst

| Aspect | Native USDT (this) | libstabst |
|--------|-------------------|-----------|
| Dependency | Custom Go fork | CGO + libstapsdt |
| Stdlib coverage | Automatic | Manual probe placement |
| Probe creation | Compile-time | Runtime |
| Maintenance | Requires Go fork | Library updates only |
| Portability | Fork-specific | Works with standard Go |
| Code changes | None (auto-instrumented) | Explicit probe calls |

## Files

- `Dockerfile` - Multi-stage build from Go fork
- `README.md` - This documentation
- `exporter/` - bpftrace to OTel exporter bridge
  - `Dockerfile` - bpftrace + exporter image
  - `main.go` - OTel export bridge
  - `trace-json.bt` - bpftrace script for stdlib probes

## Limitations

1. Requires custom Go fork (not upstream)
2. Linux only (USDT relies on ELF and kernel uprobes)
3. Privileged container needed for bpftrace attachment
4. Kernel 4.14+ required for uprobe support
5. Build time significantly longer (compiles Go from source)

## References

- [Go Fork: poc_usdt](https://github.com/kakkoyun/go/tree/poc_usdt)
- [USDT Overview](https://lwn.net/Articles/753601/)
- [bpftrace](https://github.com/iovisor/bpftrace)
- [SystemTap SDT format](https://sourceware.org/systemtap/wiki/UserSpaceProbeImplementation)
