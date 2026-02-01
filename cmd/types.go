package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
)

// RunManyOpts holds options for running multiple test scenarios.
type RunManyOpts struct {
	Logger   *slog.Logger
	Inputs   *Input
	Scenario []string
	Force    bool
	Num      int
	Timeout  time.Duration
}

// TestResult holds timing and telemetry data from a single test run.
type TestResult struct {
	Start      time.Time                  `json:"start"`
	AppStart   time.Time                  `json:"app_start"`
	AppReady   time.Time                  `json:"app_ready"`
	LoadStart  time.Time                  `json:"load_start"`
	LoadEnd    time.Time                  `json:"load_end"`
	StopStart  time.Time                  `json:"stop_start"`
	StopEnd    time.Time                  `json:"stop_end"`
	Requests   []Request                  `json:"requests"`
	LoadStats  []*container.StatsResponse `json:"load_stats"`
	StopStats  []*container.StatsResponse `json:"stop_stats"`
	Profiles   []*ProfilePayload          `json:"profiles"`
	Traces     []*TracesPayload           `json:"traces"`
	Logs       []*LogPayload              `json:"logs"`
	LoopsNum   int                        `json:"loops_num"`
	AllocsNum  int                        `json:"allocs"`
	RunnerOS   string                     `json:"runner_os,omitempty"`
	RunnerArch string                     `json:"runner_arch,omitempty"`
	RunnerCPU  int                        `json:"runner_cpu,omitempty"`
}

// Request holds timing data for a single HTTP request.
type Request struct {
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error"`
}

// ProfilePayload holds profiling data collected during a test.
type ProfilePayload struct {
	Error error    `json:"error"`
	Files []string `json:"files"`
	Bytes int64    `json:"bytes"`
}

// TracesPayload holds trace data collected during a test.
type TracesPayload struct {
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
	Trace []byte `json:"trace"`
}

// LogPayload holds log data collected during a test.
type LogPayload struct {
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
	Logs  []byte `json:"logs"`
}

// Client wraps the Docker client.
type Client struct {
	*docker.Client
}

// BuildOpts holds Docker build options.
type BuildOpts struct {
	// Dir points to the directory containing the Dockerfile.
	Dir string
	// ContextDir overrides the build context directory (defaults to project root).
	ContextDir string
	// https://docs.docker.com/reference/cli/docker/buildx/build/#build-arg
	Args    map[string]string
	Secrets map[string]string
}

// Input holds experiment configuration parameters.
type Input struct {
	// Hash of all inputs except for the hash itself.
	Hash string `json:"hash"`

	// Archetype used to generate default values (if any). For example, "idle", "throughput", "latency", or "enterprise".
	Archetype string `json:"archetype,omitempty"`

	// RuntimeVersion to test. For example, "1.24".
	RuntimeVersion string `json:"runtime_version"`

	// Workers is the number of workers to use. The exact meaning of this
	// depends on the language. For example, in Go it's GOMAXPROCS. In node it's
	// the number of cluster processes to spawn. The value of workers defaults
	// to the number of logical CPUs on the machine.
	Workers int `json:"workers"`

	// LoopsCPU (in seconds) is the amount of CPU time to spend in a tight for
	// loop in the request handler for each request. If set, the value of Loops
	// will be ignored and overwritten with an auto-calibrated value.
	LoopsCPU float64 `json:"loops_cpu"`

	// LoopsNum is the number of iterations to perform in a tight loop.
	LoopsNum int `json:"loops_num"`

	// AllocsCPU (in seconds) is the amount of CPU time to spend in a for loop
	// doing allocations of allocs_size bytes in the request handler for each
	// request. The allocations are kept alive for the duration of the request.
	// If set, the value of AllocsNum will be ignored and overwritten with an
	// auto-calibrated value.
	AllocsCPU float64 `json:"allocs_cpu"`

	// AllocsNum is the number of allocations to perform in a tight loop.
	AllocsNum int `json:"allocs_num"`

	// AllocsSize is the size of the allocations to perform in a tight loop.
	AllocsSize int `json:"allocs_size"`

	// Exceptions controls whether to throw an exception or not in the request handler.
	Exceptions bool `json:"exceptions"`

	// OffCPU time (in seconds) to spend in the request handler for each request.
	OffCPU float64 `json:"off_cpu"`

	// Recursive refers to the Stack: should it be unique functions or a recursive function call.
	Recursive bool `json:"recursive"`

	// Flush controls whether the application is asked to gracefully shut down
	// and flush traces and profiles. When disabled, the application is killed
	// via SIGKILL and buffered traces and profiles are discarded.
	Flush bool `json:"flush"`

	// Port to listen on for the application.
	Port int `json:"port"`

	// RPS is the number of requests per second to generate in an open loop.
	// This option is mutually exclusive with Concurrency.
	RPS int `json:"rps"`

	// Clients is the number of goroutines that will generated requests in a
	// closed loop. This option is mutually exclusive with RPS.
	Clients int `json:"clients"`

	// Timeout for each request in seconds.
	Timeout float64 `json:"timeout"`

	// Duration for which to put the application under load in seconds.
	Duration float64 `json:"duration"`

	// OtelEndpoint is the OpenTelemetry collector endpoint (e.g. "otel-collector:4318")
	OtelEndpoint string `json:"otel_endpoint"`
}

// NewClient creates a new Docker client.
func NewClient(_ context.Context) (*Client, error) {
	c, err := docker.NewClientWithOpts()
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

// BuildCommand constructs a docker build command for the given scenario.
func (c *Client) BuildCommand(ctx context.Context, opts *BuildOpts, scenario string) *exec.Cmd {
	args := []string{
		"build",
		"-t", scenario,
		"-f", filepath.Join(opts.Dir, "Dockerfile"),
	}
	for k, v := range opts.Args {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	for id, env := range opts.Secrets {
		args = append(args, "--secret", fmt.Sprintf("id=%s,env=%s", id, env))
	}
	// Use ContextDir if specified, otherwise default to project root
	contextDir := getRoot()
	if opts.ContextDir != "" {
		contextDir = opts.ContextDir
	}
	args = append(args, contextDir)
	return exec.CommandContext(ctx, "docker", args...)
}
