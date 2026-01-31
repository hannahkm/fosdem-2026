# Frida-based Dynamic Instrumentation Implementation Notes

## What We Achieved

### Successfully Implemented

1. **Frida Attachment** - Attached to running Go process via PID namespace sharing
2. **Symbol Resolution** - Located `main.HealthHandler` at runtime using debug symbols  
3. **Memory Allocation** - Created RWX trampoline memory and 8MB alternate stack
4. **ARM64 Assembly** - Wrote complete assembly trampoline with stack pivoting and register saving
5. **Memory Patching** - Directly patched function bytes to insert JUMP instruction (bypassing Frida's Interceptor API)
6. **Hook Execution** - Successfully intercepted first HTTP request
7. **Request Pointer Extraction** - Retrieved `*http.Request` pointer from Go's register-based ABI
8. **OTLP Pipeline** - Complete trace export: Frida → Python Bridge → OTel Collector → Jaeger
9. **Trace Verification** - Real trace appeared in Jaeger: "HTTP UNKNOWN /health"

### Partially Working

1. **HTTP Method Extraction** - Request pointer obtained but Method field reading fails (garbage length values)
2. **Single Interception** - Hook fires once then app crashes
3. **Return Path** - Trampoline executes but doesn't properly return to original function

## Technical Findings

### Go Struct Offsets (Go 1.25.6)

From `find-offsets.go`:
- `Method`: offset 0 bytes (string: 16 bytes)
- `RequestURI`: offset 192 bytes (string: 16 bytes)
- `URL`: offset 16 bytes
- `Host`: offset 128 bytes

### ARM64 Register-Based ABI

For `func HealthHandler(w http.ResponseWriter, r *http.Request)`:
- `x0` = ResponseWriter (interface)
- `x1` = *http.Request pointer

Our trampoline successfully:
- Moved `x1` → `x0` (ABI switch for C calling convention)
- Called NativeCallback with request pointer
- Pointer value confirmed valid (e.g., `0x40000b24b0`)

### Why Method Extraction Failed

Received invalid length: `274877908544` (0x40_0001_6870)

**Hypothesis**: The request pointer might be pointing to a different struct layout than expected, or we're reading at the wrong alignment. The length value looks like it might be reading pointer bytes as length.

### Why App Crashes After First Call

**Root cause**: Trampoline return path incomplete

**What happens:**
1. JUMP patches first 8 bytes of `HealthHandler`  
2. Trampoline executes, calls our JS handler
3. Attempts to return via `br x16` to `targetAddr + 8`
4. But original function prologue is corrupted
5. Stack/register state may be invalid
6. App segfaults

**What's needed:**
1. Backup the overwritten instructions properly
2. Relocate them to trampoline
3. Execute backup instructions before returning
4. Calculate correct return address
5. Handle instruction relocation (PC-relative instructions need patching)

## Comparison with Other Instrumentation Methods

### Implementation Complexity

| Method | Code Changes | Build Process | Runtime Injection | Assembly Required | Concurrency Safe |
|--------|--------------|---------------|-------------------|-------------------|------------------|
| **Manual** | Heavy (explicit OTel SDK) | Standard | No | No | Yes |
| **eBPF** | None | Standard | Via kernel | No (kernel handles it) | Yes |
| **OBI** | None | Standard | Via kernel | No | Yes |
| **Orchestrion** | None (generated) | Custom (`orchestrion go build`) | No | No | Yes |
| **Injector (Full)** | None | Standard | Yes | **Yes (complex)** | **No (needs semaphores)** |
| **Injector (Current)** | None | Standard | Yes | Partial | No |

### What Each Method Can Instrument

| Method | HTTP Method | HTTP URI | Custom Headers | Request Body | Response Status | Timing |
|--------|-------------|----------|----------------|--------------|-----------------|--------|
| **Manual** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **eBPF** | ✅ | ✅ | ⚠️ Limited | ❌ | ✅ | ✅ |
| **OBI** | ✅ | ✅ | ⚠️ Limited | ❌ | ✅ | ✅ |
| **Orchestrion** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Injector (Full)** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Injector (Current)** | ❌ | ⚠️ Hardcoded | ❌ | ❌ | ❌ | ⚠️ Partial |

### Performance Overhead

Based on methodology (actual benchmarks would be needed):

| Method | Expected Overhead | Why |
|--------|-------------------|-----|
| **Manual** | ~5-10% | Direct SDK calls, optimized by compiler |
| **eBPF** | <1% | Kernel-level, minimal context switch |
| **OBI** | <1% | Kernel-level eBPF instrumentation |
| **Orchestrion** | ~5-10% | Compile-time injection, similar to manual |
| **Injector (Full)** | ~20-40% | Stack pivoting, ABI switching, register saving |

### Production Readiness

| Method | Production Ready | Why / Why Not |
|--------|------------------|---------------|
| **Manual** | ✅ Yes | Officially supported, stable |
| **eBPF** | ✅ Yes | Mature, used in production |
| **OBI** | ⚠️ Beta | Newer, less battle-tested |
| **Orchestrion** | ✅ Yes | Datadog production tool |
| **Injector** | ❌ No | PoC only - crashes after first call, no concurrency safety |

## What Would Be Needed for Production

### Critical Issues to Fix

1. **Proper Instruction Backup and Relocation**
   ```javascript
   // Need to:
   - Disassemble original function prologue
   - Identify PC-relative instructions
   - Patch offsets when relocating
   - Execute in trampoline before returning
   ```

2. **Correct Return Address Calculation**
   ```javascript
   // Must account for:
   - Instruction length of patched bytes
   - Stack frame setup
   - Link register state
   ```

3. **Concurrency Safety**
   ```javascript
   // Add spinlock/semaphore:
   MUTEX_ACQUIRE:
       ldxr w9, [mutex_addr]
       cbnz w9, MUTEX_ACQUIRE
       mov w9, #1
       stxr w10, w9, [mutex_addr]
       cbnz w10, MUTEX_ACQUIRE
   ```

4. **Multi-Architecture Support**
   - x86-64 trampoline (different registers: RAX, RBX vs x0, x1)
   - Different instruction encodings
   - Different stack conventions

5. **Robust Struct Parsing**
   - Handle Go version differences
   - Validate pointer sanity before dereferencing
   - Handle nil/empty strings gracefully

### Estimated Additional Effort

- Fix return path: 4-6 hours
- Add concurrency safety: 2-3 hours
- Multi-architecture: 3-4 hours
- Robust error handling: 2-3 hours
- **Total: 11-16 additional hours**

## Current State: Valuable PoC

### What This PoC Demonstrates

✅ **Concept Validation**
- Dynamic injection into unmodified Go binary IS possible
- Memory patching works
- Frida can attach to containerized Go processes
- Real HTTP interception fires (once)
- Complete export pipeline functions

✅ **Architecture Understanding**
- Go's register-based ABI documented
- Struct field offsets identified
- Trampoline mechanics understood
- Integration with existing infrastructure proven

⚠️ **Known Limitations**
- Single-shot interception (crashes after first call)
- Method extraction broken (wrong data interpretation)
- No concurrency safety
- ARM64 only (would need x86-64 version)

## Value for FOSDEM Talk

This partial implementation is **ideal for a conference presentation** because:

1. **Shows the Challenge** - Demonstrates why Go instrumentation is hard
2. **Proves Feasibility** - Hook fires, data is accessible
3. **Honest Discussion** - Can explain exact technical barriers
4. **Comparison Enabled** - Now have data on ALL methods (manual, eBPF, OBI, Orchestrion, Injector)
5. **Educational** - Audience learns about Go internals, ABIs, trampolines

### Recommended Presentation Flow

1. Show the working scenarios (manual, eBPF, Orchestrion)
2. Introduce Frida approach
3. Demo: Hook fires, show Jaeger trace
4. Explain: "Works once, then crashes - here's why..."
5. Deep dive: Show ARM64 assembly, explain ABI mismatch
6. Reference: Quarkslab's full solution
7. Conclude: Compare trade-offs (complexity vs. capability)

This creates a compelling narrative about the **real challenges** of zero-code instrumentation.

## References

- [Quarkslab Part 1: Hooking Go with C/Assembly](https://blog.quarkslab.com/lets-go-into-the-rabbit-hole-part-1-the-challenges-of-dynamically-hooking-golang-program.html)
- [Quarkslab Part 2: Hooking Go with CGO](https://blog.quarkslab.com/lets-go-into-the-rabbit-hole-part-2-the-challenges-of-dynamically-hooking-golang-program.html)
- [Quarkslab Working Code](https://github.com/quarkslab/hooking-golang-playground)
- [Go Register ABI Spec](https://go.googlesource.com/proposal/+/master/design/40724-register-calling.md)
- [Go Internal ABI Documentation](https://go.googlesource.com/go/+/refs/heads/dev.regabi/src/cmd/compile/internal-abi.md)
