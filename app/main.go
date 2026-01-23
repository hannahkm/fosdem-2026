package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// Input defines the subset of the doe.cue inputs implemented by this program.
type Input struct {
	Port         int     `json:"port"`
	OffCPU       float64 `json:"off_cpu"`
	LoopsCPU     float64 `json:"loops_cpu"`
	LoopsNum     int     `json:"loops_num"`
	AllocsCPU    float64 `json:"allocs_cpu"`
	AllocsNum    int     `json:"allocs_num"`
	AllocSize    int     `json:"alloc_size"`
	Tracing      bool    `json:"tracing"`
	Profiling    bool    `json:"profiling"`
	Workers      int     `json:"workers"`
	OTelEndpoint string  `json:"otel_endpoint"`
}

func processInputs() (*Input, error) {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <inputs.json>")
		os.Exit(1)
	}

	// Read the JSON inputs file.
	inputsPath := os.Args[1]
	data, err := os.ReadFile(inputsPath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
		return nil, err
	}

	// Parse the JSON into the Inputs struct.
	var inputs Input
	if err := json.Unmarshal(data, &inputs); err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
		return nil, err
	}
	return &inputs, nil
}

func main() {
	log.SetFlags(0)
	inputs, err := processInputs()
	if err != nil {
		log.Fatalf("Error processing inputs: %v", err)
		os.Exit(1)
	}

	if inputs.Workers != 0 {
		log.Printf("Setting GOMAXPROCS to %d", inputs.Workers)
		runtime.GOMAXPROCS(inputs.Workers)
	}

	mux := setupHandlers(inputs)

	// Start the HTTP server using the port specified in the inputs.
	addr := fmt.Sprintf(":%d", inputs.Port)
	server := &http.Server{Addr: addr, Handler: mux}

	// Channel to listen for interrupt signal to gracefully shutdown the server
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Run the server in a goroutine
	go func() {
		log.Printf("Starting server on %s...", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sig := <-stop
	log.Printf("Received signal %d (%s), shutting down...", sig, sig.String())

	// Shutdown the server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exiting")
}

func setupHandlers(inputs *Input) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/load/1", inputs.LoadHandler)
	return mux
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK\n")

}

func (c *Input) LoadHandler(w http.ResponseWriter, r *http.Request) {
	a := allocsLoop(c.AllocsNum, c.AllocSize)
	simulateOffCPU(c.OffCPU)
	cpuLoop(c.LoopsNum)
	runtime.KeepAlive(a)
	io.WriteString(w, "Hello World\n")

}

// cpuLoop performs a computationally expensive loop that scales with iterations
// The function uses volatile arithmetic operations that are unlikely to be
// optimized away.
func cpuLoop(iterations int) {
	// Start with some non-zero values to prevent optimization
	result := int64(0x1234)
	// Use a volatile prime number to avoid simple pattern recognition
	volatile := int64(982451653)
	for i := range iterations {
		// Mix of operations to prevent easy compiler optimizations
		result = ((result * 48271) % 2147483647) ^ volatile
		volatile = (volatile*37 + result) % 9973
		// XOR with loop counter to ensure the result depends on the loop iteration
		result ^= int64(i)
	}
}

//go:noinline
func allocsLoop(iterations int, allocSize int) allocs {
	a := allocs{slices: make([][]byte, 0, iterations)}
	for range iterations {
		a.slices = append(a.slices, make([]byte, allocSize))
	}
	return a
}

type allocs struct {
	slices [][]byte
}

func simulateOffCPU(seconds float64) {
	if seconds <= 0 {
		return
	}
	time.Sleep(time.Duration(seconds * float64(time.Second)))
}
