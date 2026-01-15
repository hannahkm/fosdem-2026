package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

var (
	dockerCommand string = "docker-compose"
	dockerClient  *Client
	serverPID     *os.Process
)

func Many(ctx context.Context, opts *RunManyOpts) ([]*TestResult, error) {
	log := opts.Logger
	results := []*TestResult{}

	// Stop and clean up environment
	if opts.Scenario[0] == "stop" {
		err := cleanup(log)
		return nil, err
	}

	if opts.Scenario[0] == "all" {
		opts.Scenario = []string{"default", "manual", "OBI", "eBPF", "orchestrion"}
	}

	err := setupEnvironment(ctx, opts)
	if err != nil {
		return nil, err
	}
	for _, s := range opts.Scenario {
		log.Info("Running scenario", "scenario", s)
		failures := 0
		for i := range opts.Num {
			log.Info("Running test run", "run", i+1, "of", opts.Num)
			r, err := runOne(ctx, opts, s)
			if err != nil {
				log.Warn("⚠️ Test run failed", "error", err)
				failures++
				continue
			}
			results = append(results, r)
		}
		log.Info("Scenario completed", "scenario", s, "failures", failures)
	}
	return results, nil
}

func runOne(ctx context.Context, opts *RunManyOpts, scenario string) (*TestResult, error) {
	log := opts.Logger
	log.Info("Starting test run")
	start := time.Now()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	out := &TestResult{
		Start:      start,
		RunnerOS:   runtime.GOOS,
		RunnerArch: runtime.GOARCH,
		RunnerCPU:  runtime.NumCPU(),
	}
	inputs := opts.Inputs

	cleanup, err := buildGoEnvironment(ctx, opts, scenario)
	if err != nil {
		return nil, err
	}

	// Wait for the app server to be healthy
	log.Info("⌛ app server waiting to be healthy")
	waitStart := time.Now()
	if err := waitForAppHealth(ctx, inputs.Port); err != nil {
		return nil, err
	}
	log.Info("✅ app server is healthy", "duration", time.Since(waitStart))
	out.AppReady = time.Now()

	// generate load
	out.LoadStart = time.Now()
	stats := startStats(ctx, scenario)

	// Send requests
	client := &http.Client{
		Timeout: time.Duration(inputs.Timeout * 1e9),
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
			MaxIdleConnsPerHost: inputs.RPS,
		},
	}
	requests, err := Generate(ctx, &Config{
		Client:      client,
		Log:         log,
		URL:         fmt.Sprintf("http://localhost:%d/load", inputs.Port),
		RPS:         inputs.RPS,
		Clients:     inputs.Clients,
		Duration:    inputs.Duration,
		Endpoints:   inputs.Endpoints,
		ExpectError: inputs.Exceptions,
	})
	if err != nil {
		return nil, err
	}
	out.Requests = requests

	out.LoadEnd = time.Now()
	out.LoadStats, err = stats()
	if err != nil {
		return nil, err
	}

	stopStats := startStats(ctx, scenario)

	out.StopStart = time.Now()
	signal := ""
	if !inputs.Flush {
		signal = "SIGKILL"
	}
	err = cleanup(container.StopOptions{Signal: signal})
	if err != nil {
		return nil, err
	}
	out.StopEnd = time.Now()
	out.StopStats, err = stopStats()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func waitForAppHealth(ctx context.Context, port int) error {
	healthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-healthCtx.Done():
			return context.Cause(healthCtx)
		case <-time.After(100 * time.Millisecond):
		}

		if healthy, _ := httpHealthCheck(healthCtx, port); healthy {
			break
		}
	}
	return nil
}

func httpHealthCheck(ctx context.Context, port int) (bool, error) {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}
	url := fmt.Sprintf("http://localhost:%d/health", port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func setupEnvironment(ctx context.Context, opts *RunManyOpts) error {
	log := opts.Logger

	// Check if Docker is installed
	err := exec.Command("docker", "compose", "version").Run()
	if cmdExists("docker-compose") {
		dockerCommand = "docker-compose"
	} else if err == nil {
		dockerCommand = "docker compose"
	} else {
		return errors.New("Neither `docker-compose` nor `docker compose` was found. Please install one of them.")
	}

	dockerClient, err = NewClient(ctx)
	if err != nil {
		return err
	}

	// Setup Docker services
	run(dockerCommand, "up", "-d", "--remove-orphans")
	time.Sleep(3 * time.Second)
	run(dockerCommand, "ps")
	log.Info("✅ Services started!")
	log.Info("   - Grafana: http://localhost:3000")
	log.Info("   - InfluxDB: http://localhost:8086")
	log.Info("   - Jaeger: http://localhost:16686")
	log.Info("   - Prometheus: http://localhost:9090")

	log.Info("✅ Docker environment setup complete")
	return nil
}

func buildGoEnvironment(ctx context.Context, opts *RunManyOpts, scenario string) (func(container.StopOptions) error, error) {
	// Build the Go application
	log := opts.Logger
	buildArgs := map[string]string{}

	// First, load all variables from .env file
	envFile := filepath.Join(".env")
	if envBuildArgs, err := godotenv.Read(envFile); err == nil {
		for k, v := range envBuildArgs {
			buildArgs[k] = v
		}
	}

	// build the Dockerfile for the given scenario
	build := &BuildOpts{
		Dir:  filepath.Join(getRoot(), "app", scenario),
		Args: buildArgs,
		Secrets: map[string]string{
			"github_token": "GITHUB_TOKEN",
		},
	}
	var eg errgroup.Group
	eg.Go(func() error {
		log.Info("⌛ image build starting", "scenario", scenario)
		start := time.Now()
		cmdLog := log.With("scenario", scenario)
		buildCmd := dockerClient.BuildCommand(ctx, build)
		buildCmd.Env = os.Environ()
		if err := buildCmd.Run(); err != nil {
			return err
		}
		cmdLog.Info("✅ image build done", "duration", time.Since(start))
		return nil
	})

	cleanup := func(opts container.StopOptions) error {
		return dockerClient.ContainerStop(ctx, scenario, opts)
	}
	return cleanup, nil
}

func setupOTelEnvironment(log *slog.Logger) error {
	// Setup OTel Collector
	run(dockerCommand, "up", "-d", "otel-collector")
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	log.Info("✅ OTel environment setup complete")
	return nil
}

func setupOrchestrion(log *slog.Logger) error {
	// Check if Orchestrion is already installed
	if cmdExists("orchestrion") {
		log.Info("✅ Orchestrion is already installed")
		return nil
	}

	// Install Orchestrion
	log.Info("⚠️ Orchestrion command not found, installing latest...")
	err := run("go", "install", "github.com/orchestrion/orchestrion@latest")
	if err != nil {
		log.Error("❌ Failed to install Orchestrion")
		return err
	}
	log.Info("✅ Orchestrion setup complete")
	return nil
}

func setupEBPFEnvironment(log *slog.Logger) error {
	// Setup eBPF environment
	run(dockerCommand, "--profile", "with-auto-instrumentation", "up", "-d", "--remove-orphans")

	log.Info("✅ eBPF environment setup complete")
	return nil
}

// Cleanup running services and ports. Only run when requested by the user, since this will
// kill any local services (ie Grafana)
func cleanup(log *slog.Logger) error {
	log.Info("🧹 Cleaning up...")
	// Cleanup Docker services
	run(dockerCommand, "down", "--remove-orphans")

	// Kill ports
	if serverPID != nil {
		serverPID.Kill()
		serverPID.Wait()
	}

	log.Info("Clean up complete! ✨")
	return nil
}

func cmdExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func getRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Failed to get caller information")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func startStats(ctx context.Context, containerID string) func() ([]*container.StatsResponse, error) {
	resultCh := make(chan []*container.StatsResponse, 1)
	firstCh := make(chan struct{})
	stopCh := make(chan struct{})
	var err error
	go func() {
		snapshots := []*container.StatsResponse{}
		next := time.Now()
		for running := true; running; {
			select {
			case <-stopCh:
				// capture one last stats snapshot before exiting
				running = false
			case <-ctx.Done():
				running = false
			case <-time.After(time.Until(next)):
			}
			var stats container.StatsResponse
			stats, err = getContainerStats(ctx, containerID)
			snapshots = append(snapshots, &stats)
			next = next.Add(100 * time.Millisecond)
			select {
			case <-firstCh:
			default:
				close(firstCh)
			}
		}
		resultCh <- snapshots
	}()

	// Wait for the first stats snapshot to be taken before returning.
	<-firstCh

	return func() ([]*container.StatsResponse, error) {
		stopCh <- struct{}{}
		res := <-resultCh
		return res, err
	}
}

func getContainerStats(ctx context.Context, containerID string) (container.StatsResponse, error) {
	statsReader, err := dockerClient.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return container.StatsResponse{}, err
	}
	var stats container.StatsResponse
	if err := json.NewDecoder(statsReader.Body).Decode(&stats); err != nil {
		return container.StatsResponse{}, err
	}
	return stats, nil
}
