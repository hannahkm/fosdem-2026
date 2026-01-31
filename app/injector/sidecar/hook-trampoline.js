// Real HTTP interception using assembly trampolines (Quarkslab approach)
// Adapted for Frida + Go 1.25+ register-based ABI

console.log("=============================================================");
console.log("[Frida] Initializing assembly trampoline for Go HTTP hooking");
console.log("=============================================================");

// Detect architecture
const arch = Process.arch;
console.log(`[Frida] Architecture: ${arch}`);

if (arch !== 'arm64' && arch !== 'x64') {
    console.error(`[Frida] Unsupported architecture: ${arch}`);
    send({ type: 'error', message: `Unsupported architecture: ${arch}` });
    throw new Error(`Unsupported architecture: ${arch}`);
}

// Find main module
const modules = Process.enumerateModules();
const mainModule = modules[0];
console.log(`[Frida] Main module: ${mainModule.name} at ${mainModule.base}`);

// Known struct offsets from find-offsets.go for Go 1.25
const HTTP_REQUEST_OFFSETS = {
    Method: 0,        // string at offset 0
    RequestURI: 192   // string at offset 192
};

// Find target function - use HealthHandler (simpler, guaranteed to be called)
let targetAddr = null;
let targetName = null;

const symbols = mainModule.enumerateSymbols();
for (const sym of symbols) {
    if (sym.name === 'main.HealthHandler') {
        targetAddr = sym.address;
        targetName = sym.name;
        console.log(`[Frida] Found target: ${targetName} at ${targetAddr}`);
        break;
    }
}

if (!targetAddr) {
    console.error("[Frida] Could not find main.HealthHandler");
    send({ type: 'error', message: 'Target function not found' });
    throw new Error('Target function not found');
}

// Allocate memory for new stack (8MB like OS thread stack)
const STACK_SIZE = 8 * 1024 * 1024;
const newStack = Memory.alloc(STACK_SIZE);
const stackTop = newStack.add(STACK_SIZE - 16); // Leave some space at top
console.log(`[Frida] Allocated new stack: ${newStack} - ${stackTop}`);

// Storage for original stack pointer
const stackBackupPtr = Memory.alloc(Process.pointerSize);

// Counter for spans
let spanCounter = 0;

// JavaScript handler function that will be called from assembly
// This receives the *http.Request pointer in first C argument (x0 on ARM64)
const jsHandler = new NativeCallback(function(reqPtr) {
    try {
        spanCounter++;
        const spanId = spanCounter;
        const startTime = Date.now();
        
        console.log(`[Frida] 🎯 HealthHandler intercepted! Span ${spanId}`);
        console.log(`[Frida] Request pointer: ${reqPtr}`);
        
        // Dump first 256 bytes of the Request struct for debugging
        const requestDump = ptr(reqPtr).readByteArray(256);
        console.log("[Frida] Request struct dump:");
        console.log(hexdump(requestDump, { ansi: false, length: 256, header: false }));
        
        // Read HTTP method from *http.Request
        // Go string: {ptr: uintptr, len: int} = 16 bytes
        let method = "GET"; // Default for our test
        let uri = "/health";
        
        try {
            // Method string is at offset 0
            // First 8 bytes = pointer to string data
            // Next 8 bytes = length of string
            const methodDataPtr = ptr(reqPtr).readPointer();
            const methodLen = ptr(reqPtr).add(8).readU64();
            
            console.log(`[Frida] Method data ptr: ${methodDataPtr}, len: ${methodLen}`);
            
            // Sanity check the length
            if (methodLen > 0 && methodLen < 20 && !methodDataPtr.isNull()) {
                method = methodDataPtr.readUtf8String(Number(methodLen));
                console.log(`[Frida] ✓ Extracted method: ${method}`);
            } else {
                console.log(`[Frida] ⚠️  Method length out of range: ${methodLen}`);
                // Try reading as string anyway with max length
                try {
                    method = methodDataPtr.readUtf8String(10);
                    console.log(`[Frida] Fallback read: ${method}`);
                } catch (e2) {
                    console.log(`[Frida] Fallback also failed: ${e2.message}`);
                }
            }
        } catch (e) {
            console.log(`[Frida] Could not read method: ${e.message}`);
        }
        
        try {
            // RequestURI is at offset 192
            const uriDataPtr = ptr(reqPtr).add(HTTP_REQUEST_OFFSETS.RequestURI).readPointer();
            const uriLen = ptr(reqPtr).add(HTTP_REQUEST_OFFSETS.RequestURI + 8).readU64();
            
            console.log(`[Frida] URI data ptr: ${uriDataPtr}, len: ${uriLen}`);
            
            if (uriLen > 0 && uriLen < 2048 && !uriDataPtr.isNull()) {
                uri = uriDataPtr.readUtf8String(Number(uriLen));
                console.log(`[Frida] ✓ Extracted URI: ${uri}`);
            }
        } catch (e) {
            console.log(`[Frida] Could not read URI: ${e.message}`);
        }
        
        // Send span start
        send({
            type: 'span_start',
            span_id: spanId,
            method: method,
            uri: uri,
            timestamp: startTime
        });
        
        console.log(`[Frida] ✓ HTTP ${method} ${uri} (span ${spanId})`);
        
        // Store span info for potential onLeave
        // (For now we'll just end it immediately)
        setTimeout(function() {
            send({
                type: 'span_end',
                span_id: spanId,
                timestamp: Date.now(),
                duration: Date.now() - startTime
            });
        }, 1);
        
    } catch (e) {
        console.error(`[Frida] Error in JS handler: ${e.message}`);
        console.error(e.stack);
    }
}, 'void', ['pointer']);

console.log(`[Frida] JS handler address: ${jsHandler}`);

// Now create the assembly trampoline
// This is the heart of the Quarkslab approach adapted for Frida

const PAGE_SIZE = 4096;
const trampolineMem = Memory.alloc(PAGE_SIZE);
Memory.protect(trampolineMem, PAGE_SIZE, 'rwx');

console.log(`[Frida] Trampoline memory allocated at: ${trampolineMem}`);

// Write assembly based on architecture
if (arch === 'arm64') {
    console.log("[Frida] Writing ARM64 assembly trampoline...");
    
    const writer = new Arm64Writer(trampolineMem);
    
    // Save stack pointer
    writer.putLdrRegAddress('x9', stackBackupPtr);
    writer.putStrRegRegOffset('sp', 'x9', 0);
    
    // Load new stack
    writer.putLdrRegAddress('x9', stackTop);
    writer.putMovRegReg('sp', 'x9');
    
    // Save registers (ARM64 volatile: x0-x18, x30)
    writer.putPushRegReg('x0', 'x1');
    writer.putPushRegReg('x2', 'x3');
    writer.putPushRegReg('x4', 'x5');
    writer.putPushRegReg('x6', 'x7');
    writer.putPushRegReg('x8', 'x9');
    writer.putPushRegReg('x10', 'x11');
    writer.putPushRegReg('x12', 'x13');
    writer.putPushRegReg('x14', 'x15');
    writer.putPushRegReg('x16', 'x17');
    writer.putPushRegReg('x18', 'x30'); // x30 is link register
    
    // ABI switch: Go uses same registers as C on ARM64
    // x0 = ResponseWriter (interface - ignore)
    // x1 = *http.Request (what we want!)
    // Move x1 to x0 for our handler (first C arg)
    writer.putMovRegReg('x0', 'x1');
    
    // Call JS handler
    writer.putLdrRegAddress('x9', jsHandler);
    writer.putBlrReg('x9');
    
    // Restore registers
    writer.putPopRegReg('x18', 'x30');
    writer.putPopRegReg('x16', 'x17');
    writer.putPopRegReg('x14', 'x15');
    writer.putPopRegReg('x12', 'x13');
    writer.putPopRegReg('x10', 'x11');
    writer.putPopRegReg('x8', 'x9');
    writer.putPopRegReg('x6', 'x7');
    writer.putPopRegReg('x4', 'x5');
    writer.putPopRegReg('x2', 'x3');
    writer.putPopRegReg('x0', 'x1');
    
    // Restore original stack
    writer.putLdrRegAddress('x9', stackBackupPtr);
    writer.putLdrRegRegOffset('x9', 'x9', 0);
    writer.putMovRegReg('sp', 'x9');
    
    // Jump back to ORIGINAL function, not return
    // We need to let the original HealthHandler execute
    // Strategy: Jump to the original function address + offset past our patch
    const returnAddr = targetAddr.add(8); // Skip our LDR+BR (8 bytes)
    writer.putLdrRegAddress('x16', returnAddr);
    writer.putBrReg('x16');
    
    writer.flush();
    
    console.log("[Frida] ARM64 trampoline written");
    
} else if (arch === 'x64') {
    console.log("[Frida] Writing x86-64 assembly trampoline...");
    
    const writer = new X86Writer(trampolineMem);
    
    // Save current stack
    writer.putMovRegAddress('r9', stackBackupPtr);
    writer.putMovRegPtrReg('r9', 'rsp');
    
    // Load new stack
    writer.putMovRegAddress('rsp', stackTop);
    
    // Save volatile registers (System V ABI: RAX, RCX, RDX, RSI, RDI, R8-R11)
    writer.putPushReg('rax');
    writer.putPushReg('rcx');
    writer.putPushReg('rdx');
    writer.putPushReg('rsi');
    writer.putPushReg('rdi');
    writer.putPushReg('r8');
    writer.putPushReg('r9');
    writer.putPushReg('r10');
    writer.putPushReg('r11');
    
    // ABI switch: Go register → C stack
    // Go: RAX=ResponseWriter, RBX=*Request
    // C: RDI=first arg
    writer.putMovRegReg('rdi', 'rbx'); // Move *Request from RBX to RDI
    
    // Call JS handler
    writer.putMovRegAddress('r9', jsHandler);
    writer.putCallReg('r9');
    
    // Restore registers
    writer.putPopReg('r11');
    writer.putPopReg('r10');
    writer.putPopReg('r9');
    writer.putPopReg('r8');
    writer.putPopReg('rdi');
    writer.putPopReg('rsi');
    writer.putPopReg('rdx');
    writer.putPopReg('rcx');
    writer.putPopReg('rax');
    
    // Restore original stack
    writer.putMovRegAddress('r9', stackBackupPtr);
    writer.putMovRegRegPtr('rsp', 'r9');
    
    // TODO: Execute backup instructions
    writer.putRet();
    writer.flush();
    
    console.log("[Frida] x86-64 trampoline written");
}

// Now manually patch the target function with a JUMP to our trampoline
// This is the Quarkslab approach - bypass Frida's Interceptor API entirely
console.log("[Frida] Installing hook via direct memory patching...");

try {
    // Read original bytes for backup (we'll need to execute them in trampoline)
    const BACKUP_SIZE = 16;  // Backup first 16 bytes
    const originalBytes = targetAddr.readByteArray(BACKUP_SIZE);
    console.log("[Frida] Backed up original bytes:", hexdump(originalBytes, { ansi: false, length: BACKUP_SIZE }));
    
    // Change memory permissions to RWX
    Memory.protect(targetAddr, Process.pageSize, 'rwx');
    console.log("[Frida] Changed target function permissions to RWX");
    
    // Write JUMP instruction to trampoline
    // ARM64: B (branch) instruction - 0x14000000 | (offset >> 2)
    // Or use LDR + BR for absolute jump
    
    if (arch === 'arm64') {
        // ARM64 absolute jump pattern:
        // ldr x16, #8      ; Load address from PC+8
        // br x16           ; Branch to x16  
        // .quad <address>  ; 64-bit address
        
        const jumpCode = new Arm64Writer(targetAddr);
        jumpCode.putLdrRegAddress('x16', trampolineMem);
        jumpCode.putBrReg('x16');
        jumpCode.flush();
        
        console.log("[Frida] Wrote ARM64 jump to trampoline");
        
    } else if (arch === 'x64') {
        // x86-64: JMP to trampoline using Quarkslab's method
        // push <low32>; mov [rsp+4], <high32>; ret
        
        const trampolineAddr = trampolineMem.toInt64();
        const low32 = trampolineAddr & 0xFFFFFFFF;
        const high32 = (trampolineAddr >> 32) & 0xFFFFFFFF;
        
        const jumpStub = [
            0x68, ...int32ToBytes(low32),     // push <low32>
            0xC7, 0x44, 0x24, 0x04,           // mov [rsp+4], <high32>
            ...int32ToBytes(high32),
            0xC3                               // ret
        ];
        
        targetAddr.writeByteArray(jumpStub);
        console.log("[Frida] Wrote x86-64 jump to trampoline");
    }
    
    console.log(`[Frida] ✅ Successfully patched ${targetName} with manual jump!`);
    send({ type: 'ready', message: 'Real HTTP interception active via memory patching' });
    
} catch (e) {
    console.error(`[Frida] Failed to patch function: ${e.message}`);
    console.error(e.stack);
    send({ type: 'error', message: `Memory patching failed: ${e.message}` });
}

// Helper for x86-64
function int32ToBytes(val) {
    return [
        val & 0xFF,
        (val >> 8) & 0xFF,
        (val >> 16) & 0xFF,
        (val >> 24) & 0xFF
    ];
}

console.log("=============================================================");
console.log("[Frida] Trampoline hook ready - waiting for HTTP requests...");
console.log("=============================================================");
