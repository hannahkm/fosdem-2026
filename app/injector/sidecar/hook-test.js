// Test hook that creates synthetic spans to verify the pipeline works
console.log("=============================================================");
console.log("[Frida] Test mode: Creating synthetic spans...");
console.log("=============================================================");

// Find the main module to verify we're attached
const modules = Process.enumerateModules();
const mainModule = modules[0];
console.log(`[Frida] Attached to: ${mainModule.name} at ${mainModule.base}`);

// Create synthetic spans every 2 seconds to test the pipeline
let spanCounter = 0;
send({ type: 'ready', message: 'Test mode - generating synthetic spans' });

setInterval(function() {
    spanCounter++;
    const spanId = spanCounter;
    const startTime = Date.now();
    
    console.log(`[Frida] Creating test span ${spanId}...`);
    
    // Send span start
    send({
        type: 'span_start',
        span_id: spanId,
        method: 'GET',
        uri: `/test-${spanId}`,
        timestamp: startTime
    });
    
    // Send span end after 100ms
    setTimeout(function() {
        send({
            type: 'span_end',
            span_id: spanId,
            timestamp: Date.now(),
            duration: Date.now() - startTime
        });
        console.log(`[Frida] Completed test span ${spanId}`);
    }, 100);
    
}, 2000);

console.log("[Frida] ✅ Test span generator running!");
