// Hook that tests if Interceptor fires and reads Go register-based ABI
console.log("=============================================================");
console.log("[Frida] Testing register-based hook for Go 1.17+...");
console.log("=============================================================");

const modules = Process.enumerateModules();
const mainModule = modules[0];
console.log(`[Frida] Main module: ${mainModule.name} at ${mainModule.base}`);

// Find handler functions
let loadHandlerAddr = null;
let healthHandlerAddr = null;
let serveHTTPAddr = null;

const symbols = mainModule.enumerateSymbols();
for (const sym of symbols) {
    if (sym.name.includes("LoadHandler") && !loadHandlerAddr) {
        loadHandlerAddr = sym.address;
        console.log(`[Frida] Found LoadHandler: ${sym.name} at ${sym.address}`);
    }
    if (sym.name.includes("HealthHandler") && !healthHandlerAddr) {
        healthHandlerAddr = sym.address;
        console.log(`[Frida] Found HealthHandler: ${sym.name} at ${sym.address}`);
    }
    if (sym.name.includes("serverHandler") && sym.name.includes("ServeHTTP")) {
        serveHTTPAddr = sym.address;
        console.log(`[Frida] Found ServeHTTP: ${sym.name} at ${sym.address}`);
    }
}

let spanCounter = 0;

// Simple test: just verify the hook fires WITHOUT trying to read arguments
if (serveHTTPAddr) {
    console.log("\n[Frida] Attempting to hook ServeHTTP...");
    
    try {
        Interceptor.attach(serveHTTPAddr, function() {
            // Single function style (no onEnter/onLeave object)
            // This might work better for Go functions
            spanCounter++;
            console.log(`[Frida] 🎯 ServeHTTP CALLED! Count: ${spanCounter}`);
            
            // Send a simple event
            send({
                type: 'span_start',
                span_id: spanCounter,
                method: 'GET',
                uri: `/hook-fired-${spanCounter}`,
                timestamp: Date.now()
            });
            
            // Send end event immediately
            setTimeout(function() {
                send({
                    type: 'span_end',
                    span_id: spanCounter,
                    timestamp: Date.now(),
                    duration: 1
                });
            }, 10);
        });
        
        console.log("[Frida] ✅ Hook attached using simple function style");
        send({ type: 'ready', message: 'Simple hook active' });
    } catch (e) {
        console.error(`[Frida] Failed to attach: ${e.message}`);
        console.error(e.stack);
        send({ type: 'error', message: `Hook attachment failed: ${e.message}` });
    }
} else {
    console.error("[Frida] ❌ Could not find ServeHTTP");
    send({ type: 'error', message: 'ServeHTTP not found' });
}

console.log("=============================================================");
console.log("[Frida] Hook setup complete");
console.log("=============================================================");
