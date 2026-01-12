package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var (
	dockerCommand string = "docker-compose"
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

	err := setupEnvironment(log)
	if err != nil {
		return nil, err
	}
	for _, s := range opts.Scenario {
		log.Info("Running scenario", "scenario", s)
		failures := 0
		for i := range opts.Num {
			log.Info("Running test run", "run", i+1, "of", opts.Num)
			r, err := runOne(ctx, opts)
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

func runOne(ctx context.Context, opts *RunManyOpts) (*TestResult, error) {
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

	// TODO: run tests here

	return out, nil
}

func setupEnvironment(log *slog.Logger) error {
	// Check if Docker is installed
	err := exec.Command("docker", "compose", "version").Run()
	if cmdExists("docker-compose") {
		dockerCommand = "docker-compose"
	} else if err == nil {
		dockerCommand = "docker compose"
	} else {
		return errors.New("Neither `docker-compose` nor `docker compose` was found. Please install one of them.")
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
