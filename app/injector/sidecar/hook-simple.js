// Simplified Frida hook that uses Stalker to trace execution
console.log("=============================================================");
console.log("[Frida] Starting simple HTTP tracer with Stalker...");
console.log("=============================================================");

// Find the main module
const modules = Process.enumerateModules();
const mainModule = modules[0];
console.log(`[Frida] Main module: ${mainModule.name} at ${mainModule.base}`);

// Track HTTP requests by monitoring actual handler calls
let requestCounter = 0;
const activeSpans = new Map();

// Find LoadHandler and HealthHandler
let loadHandlerAddr = null;
let healthHandlerAddr = null;

const symbols = mainModule.enumerateSymbols();
for (const sym of symbols) {
    if (sym.name.includes("LoadHandler")) {
        console.log(`[Frida] Found LoadHandler: ${sym.name} at ${sym.address}`);
        loadHandlerAddr = sym.address;
    }
    if (sym.name.includes("HealthHandler")) {
        console.log(`[Frida] Found HealthHandler: ${sym.name} at ${sym.address}`);
        healthHandlerAddr = sym.address;
    }
}

// Simple approach: Just monitor when these addresses are executed using Stalker
if (loadHandlerAddr || healthHandlerAddr) {
    console.log("[Frida] Setting up execution tracing...");
    
    // Get the main thread
    const mainThread = Process.enumerateThreads()[0];
    console.log(`[Frida] Main thread ID: ${mainThread.id}`);
    
    // Use Stalker to follow execution
    Stalker.follow(mainThread.id, {
        events: {
            call: true
        },
        onReceive: function(events) {
            // Parse events and look for our function calls
            const parser = Stalker.parse(events);
            for (const event of parser) {
                if (event[0] === 'call') {
                    const target = event[1];
                    
                    // Check if this is one of our handlers
                    if (loadHandlerAddr && target.equals(loadHandlerAddr)) {
                        requestCounter++;
                        const spanId = requestCounter;
                        const startTime = Date.now();
                        
                        console.log(`[Frida] 🎯 LoadHandler called! Request #${requestCounter}`);
                        
                        send({
                            type: 'span_start',
                            span_id: spanId,
                            method: 'GET',
                            uri: '/load',
                            timestamp: startTime
                        });
                        
                        // Simulate span end after a small delay
                        setTimeout(function() {
                            send({
                                type: 'span_end',
                                span_id: spanId,
                                timestamp: Date.now(),
                                duration: Date.now() - startTime
                            });
                        }, 100);
                    }
                    
                    if (healthHandlerAddr && target.equals(healthHandlerAddr)) {
                        requestCounter++;
                        const spanId = requestCounter;
                        const startTime = Date.now();
                        
                        console.log(`[Frida] 🎯 HealthHandler called! Request #${requestCounter}`);
                        
                        send({
                            type: 'span_start',
                            span_id: spanId,
                            method: 'GET',
                            uri: '/health',
                            timestamp: startTime
                        });
                        
                        setTimeout(function() {
                            send({
                                type: 'span_end',
                                span_id: spanId,
                                timestamp: Date.now(),
                                duration: Date.now() - startTime
                            });
                        }, 100);
                    }
                }
            }
        }
    });
    
    console.log("=============================================================");
    console.log("[Frida] ✅ Stalker tracing active!");
    console.log("[Frida] Monitoring function calls...");
    console.log("=============================================================");
    send({ type: 'ready', message: 'Stalker instrumentation active' });
} else {
    console.error("[Frida] ❌ Could not find handler functions");
    send({ type: 'error', message: 'Handler functions not found' });
}
