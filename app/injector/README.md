# Frida Dynamic Instrumentation PoC

This directory contains a proof-of-concept implementation of dynamic instrumentation for Go applications using [Frida](https://frida.re/).

## Overview

The Frida scenario demonstrates runtime instrumentation of a Go HTTP server without modifying the source code. It uses Frida's JavaScript engine to hook into Go's `net/http.serverHandler.ServeHTTP` function and exports traces to OpenTelemetry.

## Architecture

```
┌─────────────────┐
│   Go HTTP App   │  Built with -gcflags="all=-N -l"
│  (unmodified)   │  to preserve function boundaries
└────────┬────────┘
         │
         │ Frida hooks (via ptrace)
         │
┌────────▼────────┐
│  Frida Agent    │  hook.js - JavaScript hooking
│   (sidecar)     │  bridge.py - OTLP export
└────────┬────────┘
         │
         │ OTLP HTTP
         │
┌────────▼────────┐
│ OTel Collector  │  http://otel-collector:4318
└─────────────────┘
```

## Components

### 1. App Container (Dockerfile)

Builds the Go application with debug flags:

- `-gcflags="all=-N -l"`: Disables optimizations and inlining
- Ensures `net/http.serverHandler.ServeHTTP` remains intact
- Uses the standard `app/main.go` (no code changes)

### 2. Frida Sidecar (sidecar/)

**sidecar/Dockerfile**

- Python 3.11 base image
- Installs Frida tools and OpenTelemetry SDK
- Runs the bridge application

**sidecar/hook.js**

- Locates `ServeHTTP` function using debug symbols
- Hooks function entry/exit points
- Extracts HTTP method and URI from Go's `*http.Request` struct
- Sends events to bridge via Frida's messaging API

**sidecar/bridge.py**

- Attaches Frida to target process
- Receives events from hook script
- Creates OpenTelemetry spans
- Exports traces via OTLP HTTP to collector

## Technical Details

### Go Struct Parsing

The hook script reads Go's internal data structures:

```go
// Go string representation (16 bytes on 64-bit)
type string struct {
    ptr uintptr  // 8 bytes: pointer to data
    len int      // 8 bytes: length
}

// http.Request struct (simplified)
type Request struct {
    Method     string   // offset 0
    // ... other fields ...
    RequestURI string   // offset ~200 (varies by Go version)
}
```

The JavaScript hook reads these fields from memory:

```javascript
const methodPtr = reqPtr.readPointer();           // Read data pointer
const methodLen = reqPtr.add(8).readU64();        // Read length
const method = methodPtr.readUtf8String(methodLen); // Read string
```

### Container Configuration

The sidecar requires special privileges:

- `PidMode: container:frida` - Share PID namespace with app
- `CapAdd: ["SYS_PTRACE"]` - Allow process attachment
- `SecurityOpt: ["seccomp=unconfined"]` - Enable ptrace syscalls

## Usage

```bash
# Run the Frida scenario
go run . run --scenario frida --num 1

# In another terminal, generate traffic
curl http://localhost:8080/load

# View traces in Jaeger
open http://localhost:16686
```

## Comparison with Other Approaches

| Aspect | eBPF | Frida |
|--------|------|-------|
| Privilege Level | Kernel (requires eBPF) | User-space (requires ptrace) |
| Hook Points | Syscalls, uprobes | Any function |
| Performance | Very low overhead | Higher overhead |
| Flexibility | Limited to kernel events | Full control over function hooks |
| Go ABI | Handled by eBPF maps | Manual struct parsing |
| Production Ready | Yes | PoC only |

## Current Implementation Status

### What Works

- ✅ Frida attaches to Go process successfully
- ✅ Symbol resolution finds target functions  
- ✅ Memory patching injects JUMP to trampoline
- ✅ Assembly trampoline executes (ARM64)
- ✅ Hook fires on first HTTP request
- ✅ Request pointer extracted from Go registers
- ✅ Real trace appears in Jaeger ("HTTP UNKNOWN /health")
- ✅ Complete OTLP export pipeline functional

### Known Limitations

1. **Single-Shot Interception**: Hook fires once, then app crashes
   - Trampoline return path incomplete
   - Backup instructions not properly executed
   - Requires proper instruction relocation and return address calculation

2. **Method Extraction Broken**: RequestURI can be read, but Method field extraction fails
   - Receiving invalid length values (struct alignment issue)
   - Needs investigation of actual memory layout at runtime

3. **No Concurrency Safety**: Multiple goroutines would corrupt shared trampoline state
   - Requires semaphore/spinlock implementation
   - Stack pivot needs per-goroutine isolation

4. **Architecture-Specific**: ARM64 only
   - x86-64 would need different register mappings and assembly
   - RISC-V, other architectures not supported

5. **Production Gaps**:
   - No error handling
   - No request/response body inspection
   - No HTTP status code tracking
   - Missing POST/PUT/DELETE support

## Comparison with Other Methods

See [IMPLEMENTATION.md](IMPLEMENTATION.md) for detailed technical analysis and comparison tables.

### Quick Summary

| Aspect | Manual | eBPF | Orchestrion | **Injector (This PoC)** |
|--------|--------|------|-------------|-------------------------|
| **Works?** | ✅ Yes | ✅ Yes | ✅ Yes | ⚠️ Partially (1 request) |
| **Code Changes** | Heavy | None | None | None |
| **Complexity** | Low | Medium | Low | **Very High** |
| **Production Ready** | ✅ Yes | ✅ Yes | ✅ Yes | ❌ No |
| **Value** | Standard | Low overhead | Automated | **Educational** |

This PoC demonstrates **why** other approaches exist - dynamic Go instrumentation is fundamentally challenging.

## Future Improvements

- [ ] **Implement real HTTP interception**: Replace synthetic spans with actual request hooking using manual assembly trampolines that handle Go's register-based ABI (RAX, RBX, RCX for args). See Quarkslab's approach for reference.

- [ ] **Read HTTP.Request from registers**: Extract method and URI from Go's *http.Request struct by reading RAX/RBX registers instead of using Frida's args[] array.

- [ ] Dynamic offset detection (avoid hardcoded struct offsets)

- [ ] Trace context propagation from incoming headers

- [ ] Error handling and response status tracking

## References

- [Frida Documentation](https://frida.re/docs/home/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [Go's net/http internals](https://github.com/golang/go/tree/master/src/net/http)
