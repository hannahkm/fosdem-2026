#!/usr/bin/env python3
"""
Frida-to-OTLP Bridge
Attaches Frida to the target Go process and exports collected traces via OTLP.
"""

import os
import sys
import time
import signal
import frida
from typing import Dict, Any
from datetime import datetime

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.trace import Status, StatusCode


class FridaBridge:
    """Bridge between Frida instrumentation and OpenTelemetry export."""
    
    def __init__(self):
        self.session = None
        self.script = None
        self.active_spans: Dict[int, Any] = {}
        self.tracer = None
        self.running = True
        
        # Setup signal handlers for graceful shutdown
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
    
    def _signal_handler(self, signum, frame):
        """Handle shutdown signals."""
        print(f"[Bridge] Received signal {signum}, shutting down...")
        self.running = False
    
    def setup_otel(self):
        """Initialize OpenTelemetry tracer and exporter."""
        # Get OTLP endpoint from environment
        endpoint = os.environ.get('OTEL_EXPORTER_OTLP_ENDPOINT', 'http://otel-collector:4318')
        service_name = os.environ.get('OTEL_SERVICE_NAME', 'fosdem-injector')
        
        # Wait for collector to be reachable
        print(f"[Bridge] Verifying collector connectivity to {endpoint}...")
        import socket
        collector_host = endpoint.split('://')[1].split(':')[0]
        collector_port = int(endpoint.split(':')[-1].split('/')[0])
        
        for attempt in range(30):
            try:
                sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                sock.settimeout(1)
                sock.connect((collector_host, collector_port))
                sock.close()
                print(f"[Bridge] Collector is reachable at {collector_host}:{collector_port}")
                break
            except Exception as e:
                if attempt < 29:
                    if attempt % 5 == 0:
                        print(f"[Bridge] Waiting for collector... ({attempt + 1}/30)")
                    time.sleep(1)
                else:
                    print(f"[Bridge] WARNING: Could not connect to collector after 30 attempts: {e}")
                    print(f"[Bridge] Proceeding anyway - traces may not export")
        
        # Ensure endpoint has the traces path
        if not endpoint.endswith('/v1/traces'):
            endpoint = endpoint.rstrip('/') + '/v1/traces'
        
        print(f"[Bridge] Setting up OTLP exporter to {endpoint}")
        
        # Create resource with service information
        resource = Resource.create({
            "service.name": service_name,
            "service.version": "1.0.0",
            "instrumentation.provider": "frida",
        })
        
        # Setup tracer provider
        provider = TracerProvider(resource=resource)
        
        # Create OTLP exporter
        otlp_exporter = OTLPSpanExporter(
            endpoint=endpoint,
            timeout=30,
        )
        
        # Add span processor
        provider.add_span_processor(BatchSpanProcessor(otlp_exporter))
        
        # Set global tracer provider
        trace.set_tracer_provider(provider)
        
        # Get tracer
        self.tracer = trace.get_tracer("frida-instrumentation", "1.0.0")
        print("[Bridge] OpenTelemetry tracer initialized")
    
    def attach_frida(self, target_name: str = "main"):
        """Attach Frida to the target process."""
        print(f"[Bridge] Searching for target process '{target_name}'...")
        
        # In containerized environment with shared PID namespace,
        # we can directly enumerate processes
        max_retries = 30
        for attempt in range(max_retries):
            try:
                # Get local device (shared PID namespace)
                device = frida.get_local_device()
                print(f"[Bridge] Connected to device: {device}")
                
                # Enumerate processes
                processes = device.enumerate_processes()
                print(f"[Bridge] Found {len(processes)} processes")
                
                # Debug: show all processes
                if attempt == 0:
                    print("[Bridge] Available processes:")
                    for proc in processes[:20]:
                        print(f"  - {proc.name} (PID: {proc.pid})")
                
                target_process = None
                for proc in processes:
                    if target_name in proc.name:
                        target_process = proc
                        break
                
                if target_process:
                    print(f"[Bridge] Found target: {target_process.name} (PID: {target_process.pid})")
                    print(f"[Bridge] Attaching to PID {target_process.pid}...")
                    self.session = device.attach(target_process.pid)
                    print(f"[Bridge] Successfully attached to {target_process.name}")
                    break
                else:
                    if attempt < max_retries - 1:
                        if attempt % 5 == 0:
                            print(f"[Bridge] Still waiting for '{target_name}'... ({attempt + 1}/{max_retries})")
                        time.sleep(1)
                    else:
                        print(f"[Bridge] ERROR: Could not find process '{target_name}' after {max_retries} attempts")
                        print("[Bridge] Final process list:")
                        for proc in processes[:20]:
                            print(f"  - {proc.name} (PID: {proc.pid})")
                        sys.exit(1)
                        
            except Exception as e:
                if attempt < max_retries - 1:
                    if attempt % 5 == 0:
                        print(f"[Bridge] Error during attach attempt {attempt + 1}/{max_retries}: {e}")
                    time.sleep(1)
                else:
                    print(f"[Bridge] ERROR: Failed to attach after {max_retries} attempts: {e}")
                    import traceback
                    traceback.print_exc()
                    sys.exit(1)
        
        # Load the Frida hook script
        # Check for test mode
        hook_file = os.environ.get('FRIDA_HOOK_SCRIPT', '/frida/hook.js')
        print(f"[Bridge] Loading hook script from {hook_file}...")
        try:
            with open(hook_file, 'r') as f:
                script_code = f.read()
            print(f"[Bridge] Hook script loaded ({len(script_code)} bytes)")
        except Exception as e:
            print(f"[Bridge] ERROR: Failed to read hook script: {e}")
            sys.exit(1)
        
        try:
            print("[Bridge] Creating Frida script...")
            self.script = self.session.create_script(script_code)
            self.script.on('message', self._on_message)
            print("[Bridge] Loading script into target process...")
            self.script.load()
            print("[Bridge] ✅ Frida script loaded and active!")
        except Exception as e:
            print(f"[Bridge] ERROR: Failed to load script: {e}")
            import traceback
            traceback.print_exc()
            sys.exit(1)
    
    def _on_message(self, message: Dict[str, Any], data: Any):
        """Handle messages from Frida script."""
        try:
            if message['type'] == 'send':
                payload = message['payload']
                msg_type = payload.get('type')
                
                if msg_type == 'ready':
                    print(f"[Bridge] ✅ {payload.get('message', 'Ready')}")
                
                elif msg_type == 'error':
                    print(f"[Bridge] ❌ ERROR from Frida: {payload.get('message')}")
                
                elif msg_type == 'span_start':
                    self._handle_span_start(payload)
                
                elif msg_type == 'span_end':
                    self._handle_span_end(payload)
                
                else:
                    print(f"[Bridge] ⚠️  Unknown message type: {msg_type}, payload: {payload}")
            
            elif message['type'] == 'error':
                print(f"[Bridge] ❌ Frida script error:")
                print(f"  Description: {message.get('description', 'N/A')}")
                print(f"  Stack: {message.get('stack', 'N/A')}")
                print(f"  Full message: {message}")
            
            else:
                print(f"[Bridge] ℹ️  Other message type '{message['type']}': {message}")
                
        except Exception as e:
            print(f"[Bridge] ❌ Error handling message: {e}")
            print(f"  Message was: {message}")
            import traceback
            traceback.print_exc()
    
    def _handle_span_start(self, payload: Dict[str, Any]):
        """Handle span start event from Frida."""
        span_id = payload['span_id']
        method = payload.get('method', 'UNKNOWN')
        uri = payload.get('uri', '/unknown')
        timestamp = payload.get('timestamp', time.time() * 1000)
        
        # Convert milliseconds to nanoseconds for OpenTelemetry
        start_time_ns = int(timestamp * 1_000_000)
        
        # Create a span with proper context
        span = self.tracer.start_span(
            name=f"HTTP {method} {uri}",
            start_time=start_time_ns
        )
        
        # Set HTTP attributes following OpenTelemetry semantic conventions
        span.set_attribute("http.method", method)
        span.set_attribute("http.target", uri)
        span.set_attribute("http.route", uri)
        span.set_attribute("http.scheme", "http")
        span.set_attribute("span.kind", "server")
        
        # Store the span for later completion
        self.active_spans[span_id] = span
        
        print(f"[Bridge] Started span {span_id}: {method} {uri}")
    
    def _handle_span_end(self, payload: Dict[str, Any]):
        """Handle span end event from Frida."""
        span_id = payload['span_id']
        timestamp = payload.get('timestamp', time.time() * 1000)
        duration = payload.get('duration', 0)
        
        if span_id not in self.active_spans:
            print(f"[Bridge] Warning: No active span found for ID {span_id}")
            return
        
        span = self.active_spans.pop(span_id)
        
        # Set status to OK
        span.set_status(Status(StatusCode.OK))
        
        # Set duration as attribute (in milliseconds)
        span.set_attribute("duration_ms", duration)
        
        # End the span with the captured timestamp (convert ms to ns)
        end_time_ns = int(timestamp * 1_000_000)
        span.end(end_time=end_time_ns)
        
        print(f"[Bridge] Completed span {span_id} (duration: {duration}ms)")
        
        # Force flush to ensure span is exported immediately (for testing)
        try:
            trace.get_tracer_provider().force_flush(timeout_millis=1000)
        except:
            pass
    
    def run(self):
        """Main run loop."""
        print("=" * 60)
        print("[Bridge] Starting Frida-to-OTLP bridge...")
        print("=" * 60)
        
        # Setup OpenTelemetry
        print("\n[Bridge] Step 1: Setting up OpenTelemetry...")
        self.setup_otel()
        
        # Attach to target process
        target_name = os.environ.get('FRIDA_TARGET_PROCESS', 'main')
        print(f"\n[Bridge] Step 2: Attaching Frida to '{target_name}'...")
        self.attach_frida(target_name)
        
        print("\n" + "=" * 60)
        print("[Bridge] ✅ Bridge is running and monitoring!")
        print("[Bridge] Waiting for HTTP requests to instrument...")
        print("=" * 60 + "\n")
        
        # Keep the script running
        try:
            while self.running:
                time.sleep(1)
        except KeyboardInterrupt:
            print("\n[Bridge] Interrupted by user")
        
        # Cleanup
        print("\n[Bridge] Cleaning up...")
        if self.script:
            try:
                self.script.unload()
                print("[Bridge] Script unloaded")
            except:
                pass
        if self.session:
            try:
                self.session.detach()
                print("[Bridge] Session detached")
            except:
                pass
        
        # Force flush any remaining spans
        print("[Bridge] Flushing remaining spans...")
        if self.tracer:
            try:
                trace.get_tracer_provider().force_flush(timeout_millis=5000)
                print("[Bridge] Spans flushed")
            except Exception as e:
                print(f"[Bridge] Error flushing spans: {e}")
        
        print("[Bridge] Shutdown complete")


def main():
    """Main entry point."""
    bridge = FridaBridge()
    bridge.run()


if __name__ == '__main__':
    main()
