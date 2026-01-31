#!/bin/bash
set -e

echo "=== Testing Unified Exporter ==="
echo

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Configuration
COLLECTOR_ENDPOINT="${OTEL_EXPORTER_OTLP_ENDPOINT:-localhost:4318}"

echo "1. Checking if OTel Collector is running..."
if ! curl -s "http://${COLLECTOR_ENDPOINT}/v1/traces" > /dev/null 2>&1; then
    echo "Warning: OTel Collector may not be running at ${COLLECTOR_ENDPOINT}"
    echo "   Start it with: docker-compose up -d otel-collector"
fi

echo "2. Building unified exporter..."
cd "${PROJECT_ROOT}"
go build -o /tmp/unified-exporter ./app/exporter/cmd/ || {
    echo "Build failed"
    exit 1
}
echo "Build successful"
echo

echo "3. Testing libstabst mode..."
echo "   Piping test data through exporter..."

# Create test data for libstabst mode
cat > /tmp/libstabst-test.json << 'EOF'
{"event":"request_start","reqid":"unified-001","timestamp":1700000000000000000}
{"event":"request_end","reqid":"unified-001","start":1700000000000000000,"duration":5000000}
{"event":"request_start","reqid":"unified-002","timestamp":1700000001000000000}
{"event":"request_end","reqid":"unified-002","start":1700000001000000000,"duration":3000000}
EOF

# Create a test wrapper that reads from stdin
cat > /tmp/test-stdin-exporter.go << 'EOF'
package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"time"

	"fosdem2026/app/exporter/core"
	"fosdem2026/app/exporter/handlers"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()

	config := core.DefaultConfig()
	config.LoadFromEnv()
	config.Mode = core.ModeLibstabst

	exporter := core.New(config)

	if err := exporter.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}
	defer func() {
		log.Println("Shutting down...")
		if err := exporter.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down: %v", err)
		}
	}()

	spans := exporter.SpanManager()
	exporter.RegisterHandler(handlers.NewRequestHandler(spans))

	log.Printf("Reading test data from stdin (mode: %s)", config.Mode)
	log.Printf("OTel endpoint: %s", config.OTELEndpoint)

	scanner := bufio.NewScanner(os.Stdin)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := exporter.ProcessLine(ctx, line); err != nil {
			log.Printf("Error processing line %d: %v", lineNum, err)
		}
	}

	log.Printf("Processed %d lines", lineNum)
	log.Println("Waiting for spans to export...")
	time.Sleep(2 * time.Second)
}
EOF

echo "   Building stdin test exporter..."
cd "${PROJECT_ROOT}"
go build -o /tmp/test-stdin-exporter /tmp/test-stdin-exporter.go || {
    echo "Test exporter build failed"
    exit 1
}

echo "   Running test..."
OTEL_EXPORTER_OTLP_ENDPOINT="${COLLECTOR_ENDPOINT}" cat /tmp/libstabst-test.json | /tmp/test-stdin-exporter

echo
echo "4. Testing native-usdt mode..."

# Create test data for native-usdt mode
cat > /tmp/usdt-test.json << 'EOF'
{"event":"http_request_start","method":"GET","path":"/api/test","timestamp":1700000002000000000}
{"event":"http_request_end","method":"GET","path":"/api/test","status":200,"duration":10000000}
{"event":"net_dial_start","network":"tcp","address":"db:5432","timestamp":1700000003000000000}
{"event":"net_dial_end","network":"tcp","address":"db:5432","duration":2000000,"error":0}
EOF

# Create USDT test wrapper
cat > /tmp/test-usdt-exporter.go << 'EOF'
package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"time"

	"fosdem2026/app/exporter/core"
	"fosdem2026/app/exporter/handlers"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()

	config := core.DefaultConfig()
	config.LoadFromEnv()
	config.Mode = core.ModeNativeUSDT

	exporter := core.New(config)

	if err := exporter.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize exporter: %v", err)
	}
	defer func() {
		log.Println("Shutting down...")
		if err := exporter.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down: %v", err)
		}
	}()

	spans := exporter.SpanManager()
	exporter.RegisterHandler(handlers.NewHTTPHandler(spans))
	exporter.RegisterHandler(handlers.NewDialHandler(spans))
	exporter.RegisterHandler(handlers.NewTLSHandler(spans))

	log.Printf("Reading test data from stdin (mode: %s)", config.Mode)
	log.Printf("OTel endpoint: %s", config.OTELEndpoint)

	scanner := bufio.NewScanner(os.Stdin)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := exporter.ProcessLine(ctx, line); err != nil {
			log.Printf("Error processing line %d: %v", lineNum, err)
		}
	}

	log.Printf("Processed %d lines", lineNum)
	log.Println("Waiting for spans to export...")
	time.Sleep(2 * time.Second)
}
EOF

echo "   Building USDT stdin test exporter..."
go build -o /tmp/test-usdt-exporter /tmp/test-usdt-exporter.go || {
    echo "USDT test exporter build failed"
    exit 1
}

echo "   Running test..."
OTEL_EXPORTER_OTLP_ENDPOINT="${COLLECTOR_ENDPOINT}" cat /tmp/usdt-test.json | /tmp/test-usdt-exporter

echo
echo "=== Test Complete ==="
echo
echo "To verify spans in Jaeger:"
echo "  1. Open http://localhost:16686"
echo "  2. Search for services:"
echo "     - usdt-bpftrace-exporter (libstabst mode)"
echo "     - usdt-native-exporter (native-usdt mode)"
echo "  3. You should see spans:"
echo "     - unified-001, unified-002 (libstabst)"
echo "     - HTTP GET, net.Dial (native-usdt)"

# Cleanup
rm -f /tmp/test-stdin-exporter.go /tmp/test-usdt-exporter.go
rm -f /tmp/test-stdin-exporter /tmp/test-usdt-exporter
rm -f /tmp/libstabst-test.json /tmp/usdt-test.json
