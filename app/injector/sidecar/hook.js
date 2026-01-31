// Frida hook script for instrumenting Go's net/http server
// Hooks into net/http.serverHandler.ServeHTTP to capture HTTP request metadata

console.log("=" + "=".repeat(60));
console.log("[Frida] Initializing Go HTTP instrumentation...");
console.log("=" + "=".repeat(60));

// Find the base address of the main binary
const modules = Process.enumerateModules();
console.log(`[Frida] Found ${modules.length} modules`);
const mainModule = modules[0];
console.log(`[Frida] Main module: ${mainModule.name} at ${mainModule.base} (size: ${mainModule.size})`);

// Instead of hooking net/http internals, hook our actual application handlers
// These are guaranteed to be called when HTTP requests arrive
let loadHandlerAddr = null;
let healthHandlerAddr = null;

console.log("[Frida] Looking for application handler functions...");

// Try to find LoadHandler - it's a method on the Input struct
try {
    // Go mangles method names as: package.(*Type).MethodName
    const sym = DebugSymbol.fromName("main.(*Input).LoadHandler");
    if (sym && sym.address && !sym.address.isNull()) {
        loadHandlerAddr = sym.address;
        console.log("[Frida] ✓ Found LoadHandler at:", loadHandlerAddr);
    }
} catch (e) {
    console.log("[Frida] LoadHandler symbol lookup failed:", e.message);
}

// Try to find HealthHandler - it's a regular function
try {
    const sym = DebugSymbol.fromName("main.HealthHandler");
    if (sym && sym.address && !sym.address.isNull()) {
        healthHandlerAddr = sym.address;
        console.log("[Frida] ✓ Found HealthHandler at:", healthHandlerAddr);
    }
} catch (e) {
    console.log("[Frida] HealthHandler symbol lookup failed:", e.message);
}

// Fallback: search through all symbols
if (!loadHandlerAddr || !healthHandlerAddr) {
    console.log("[Frida] Searching symbols for handler functions...");
    const symbols = mainModule.enumerateSymbols();
    for (const sym of symbols) {
        if (sym.name.includes("LoadHandler")) {
            console.log(`[Frida]   Found: ${sym.name} at ${sym.address}`);
            if (!loadHandlerAddr) loadHandlerAddr = sym.address;
        }
        if (sym.name.includes("HealthHandler")) {
            console.log(`[Frida]   Found: ${sym.name} at ${sym.address}`);
            if (!healthHandlerAddr) healthHandlerAddr = sym.address;
        }
    }
}

// Try to find ServeHTTP using multiple methods
let serveHTTPAddr = null;

// Method 1: Try Module.getExportByName (for exported symbols)
try {
    serveHTTPAddr = Module.getExportByName(null, "net/http.(*serverHandler).ServeHTTP");
    if (serveHTTPAddr) {
        console.log("[Frida] Found ServeHTTP via getExportByName at:", serveHTTPAddr);
    }
} catch (e) {
    console.log("[Frida] getExportByName failed:", e.message);
}

// Method 2: Try DebugSymbol
if (!serveHTTPAddr) {
    try {
        const sym = DebugSymbol.fromName("net/http.(*serverHandler).ServeHTTP");
        if (sym && sym.address && !sym.address.isNull()) {
            serveHTTPAddr = sym.address;
            console.log("[Frida] Found ServeHTTP via DebugSymbol at:", serveHTTPAddr);
        }
    } catch (e) {
        console.log("[Frida] DebugSymbol failed:", e.message);
    }
}

// Method 3: Search symbols manually
if (!serveHTTPAddr) {
    console.log("[Frida] Searching for ServeHTTP in symbols...");
    const symbols = mainModule.enumerateSymbols();
    for (const sym of symbols) {
        if (sym.name.includes("serverHandler") && sym.name.includes("ServeHTTP")) {
            console.log(`[Frida] Found via enumeration: ${sym.name} at ${sym.address}`);
            // Convert relative address to absolute if needed
            serveHTTPAddr = mainModule.base.add(ptr(sym.address));
            console.log("[Frida] Calculated absolute address:", serveHTTPAddr);
            break;
        }
    }
}

// Attempt 3: Search for the symbol in exports/symbols
if (!serveHTTPAddr) {
    const symbols = mainModule.enumerateSymbols();
    console.log("[Frida] Searching through symbols for HTTP-related functions...");
    
    // Look for any HTTP-related symbols
    const httpSymbols = [];
    for (const sym of symbols) {
        if (sym.name.includes("http") || sym.name.includes("Handler") || sym.name.includes("Serve")) {
            httpSymbols.push(sym);
            if (sym.name.includes("serverHandler") && sym.name.includes("ServeHTTP")) {
                serveHTTPAddr = sym.address;
                console.log("[Frida] Found ServeHTTP via symbol enumeration:", sym.name, "at", serveHTTPAddr);
                break;
            }
        }
    }
    
    // Show some HTTP symbols for debugging
    if (httpSymbols.length > 0) {
        console.log(`[Frida] Found ${httpSymbols.length} HTTP-related symbols, showing first 30:`);
        for (let i = 0; i < Math.min(30, httpSymbols.length); i++) {
            console.log(`  ${i+1}. ${httpSymbols[i].name} at ${httpSymbols[i].address}`);
        }
    }
    
    // Also try looking for our specific handler functions
    console.log("[Frida] Looking for our LoadHandler and HealthHandler...");
    for (const sym of symbols) {
        if (sym.name.includes("LoadHandler") || sym.name.includes("HealthHandler")) {
            console.log(`[Frida]   - ${sym.name} at ${sym.address}`);
        }
    }
}

if (!loadHandlerAddr && !healthHandlerAddr) {
    console.error("[Frida] ❌ ERROR: Could not locate any handler functions");
    console.log("[Frida] Looking for any main.* functions...");
    
    const symbols = mainModule.enumerateSymbols();
    console.log("[Frida] Showing main package functions:");
    let mainCount = 0;
    for (const sym of symbols) {
        if (sym.name.startsWith('main.') && mainCount < 30) {
            console.log(`  - ${sym.name}`);
            mainCount++;
        }
    }
    
    send({ type: 'error', message: 'Handler functions not found' });
} else {
    console.log("[Frida] Preparing to hook handler functions...");

    // Active spans map to track request timing
    const activeSpans = new Map();
    let spanCounter = 0;
    let callCounter = 0;

    // Helper function to create hook handlers
    function createHookHandlers(handlerName) {
        return {
            onEnter: function(args) {
                callCounter++;
                console.log(`[Frida] 🎯 ${handlerName} called! (call #${callCounter})`);
                
                try {
                    // Generate span ID
                    const spanId = ++spanCounter;
                    const startTime = Date.now();
                    
                    console.log(`[Frida] Creating span ${spanId} for ${handlerName}`);
                    
                    // For Go HTTP handler: func(w http.ResponseWriter, r *http.Request)
                    // args[0] = receiver (for methods) or w (for functions)
                    // args[1] = w or r depending on whether it's a method
                    // args[2] = r (for methods)
                    
                    // Try both possibilities
                    let reqPtr = null;
                    let method = "";
                    let uri = "";
                    
                    // Determine the path from the handler name
                    if (handlerName === "HealthHandler") {
                        uri = "/health";
                        method = "GET";
                    } else if (handlerName === "LoadHandler") {
                        uri = "/load";
                        method = "GET";
                    }
                    
                    // Store span info for onLeave
                    this.spanId = spanId;
                    this.startTime = startTime;
                    activeSpans.set(spanId, { method, uri, startTime });

                    // Send span start event to bridge
                    send({
                        type: 'span_start',
                        span_id: spanId,
                        method: method,
                        uri: uri,
                        timestamp: startTime
                    });

                    console.log(`[Frida] HTTP ${method} ${uri} (span ${spanId})`);

                } catch (e) {
                    console.error("[Frida] Error in onEnter:", e.message, e.stack);
                }
            },

            onLeave: function(retval) {
                try {
                    if (this.spanId && activeSpans.has(this.spanId)) {
                        const endTime = Date.now();
                        const span = activeSpans.get(this.spanId);
                        const duration = endTime - span.startTime;

                        // Send span end event
                        send({
                            type: 'span_end',
                            span_id: this.spanId,
                            timestamp: endTime,
                            duration: duration
                        });

                        console.log(`[Frida] Completed span ${this.spanId} in ${duration}ms`);
                        activeSpans.delete(this.spanId);
                    }
                } catch (e) {
                    console.error("[Frida] Error in onLeave:", e.message);
                }
            }
        };
    }

    // Try hooking the handler functions first (they now have //go:noinline directives)
    let hookAttached = false;
    
    if (loadHandlerAddr) {
        try {
            console.log("[Frida] Attempting to attach to LoadHandler at:", loadHandlerAddr);
            Interceptor.attach(loadHandlerAddr, createHookHandlers("LoadHandler"));
            console.log("[Frida] ✅ Successfully attached to LoadHandler!");
            hookAttached = true;
        } catch (e) {
            console.log(`[Frida] Failed to attach to LoadHandler: ${e.message}`);
        }
    }
    
    if (healthHandlerAddr) {
        try {
            console.log("[Frida] Attempting to attach to HealthHandler at:", healthHandlerAddr);
            Interceptor.attach(healthHandlerAddr, createHookHandlers("HealthHandler"));
            console.log("[Frida] ✅ Successfully attached to HealthHandler!");
            hookAttached = true;
        } catch (e) {
            console.log(`[Frida] Failed to attach to HealthHandler: ${e.message}`);
        }
    }
    
    // Fallback to ServeHTTP if handler hooks failed
    if (!hookAttached && serveHTTPAddr) {
        console.log("[Frida] Falling back to ServeHTTP at:", serveHTTPAddr);
        
        // Inspect what's at this address
        try {
            const bytes = serveHTTPAddr.readByteArray(16);
            console.log("[Frida] First 16 bytes at ServeHTTP:", hexdump(bytes, { ansi: false, length: 16 }));
        } catch (e) {
            console.log("[Frida] Could not read bytes:", e.message);
        }
        
        try {
            Interceptor.attach(serveHTTPAddr, {
                onEnter: function(args) {
                    callCounter++;
                    console.log(`[Frida] 🎯 ServeHTTP called! (call #${callCounter})`);
                    
                    try {
                        const spanId = ++spanCounter;
                        const startTime = Date.now();
                        
                        // For debugging: just create a basic span
                        const method = "GET";
                        const uri = "/request-" + callCounter;
                        
                        this.spanId = spanId;
                        this.startTime = startTime;
                        activeSpans.set(spanId, { method, uri, startTime });

                        send({
                            type: 'span_start',
                            span_id: spanId,
                            method: method,
                            uri: uri,
                            timestamp: startTime
                        });

                        console.log(`[Frida] HTTP ${method} ${uri} (span ${spanId})`);

                    } catch (e) {
                        console.error("[Frida] Error in onEnter:", e.message);
                    }
                },

                onLeave: function(retval) {
                    try {
                        if (this.spanId && activeSpans.has(this.spanId)) {
                            const endTime = Date.now();
                            const span = activeSpans.get(this.spanId);
                            const duration = endTime - span.startTime;

                            send({
                                type: 'span_end',
                                span_id: this.spanId,
                                timestamp: endTime,
                                duration: duration
                            });

                            console.log(`[Frida] Completed span ${this.spanId} in ${duration}ms`);
                            activeSpans.delete(this.spanId);
                        }
                    } catch (e) {
                        console.error("[Frida] Error in onLeave:", e.message);
                    }
                }
            });
            console.log("[Frida] ✅ ServeHTTP hook attached successfully");
            hookAttached = true;
        } catch (e) {
            console.error(`[Frida] Failed to attach to ServeHTTP: ${e.message}`);
        }
    }

    if (hookAttached) {
        console.log("=".repeat(61));
        console.log("[Frida] ✅ Successfully attached to handler functions!");
        console.log("[Frida] Ready to intercept HTTP requests...");
        console.log("=".repeat(61));
        send({ type: 'ready', message: 'Instrumentation active' });
    } else {
        console.error("[Frida] ❌ Failed to attach any hooks!");
        send({ type: 'error', message: 'No hooks could be attached' });
    }
}
