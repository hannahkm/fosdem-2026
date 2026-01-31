package core

import "os"

// Mode represents the exporter operation mode.
type Mode string

const (
	ModeNativeUSDT Mode = "native-usdt"
	ModeLibstabst  Mode = "libstabst"
)

// Config holds the exporter configuration.
type Config struct {
	Mode         Mode
	OTELEndpoint string
	TargetPID    string
	BPFScript    string
	ServiceName  string
	TracerName   string
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() *Config {
	return &Config{
		Mode:         ModeLibstabst,
		OTELEndpoint: "otel-collector:4318",
		TargetPID:    "1",
		BPFScript:    "/app/trace-json.bt",
		ServiceName:  "bpftrace-exporter",
		TracerName:   "bpftrace-exporter",
	}
}

// LoadFromEnv populates the configuration from environment variables.
func (c *Config) LoadFromEnv() {
	if mode := os.Getenv("EXPORTER_MODE"); mode != "" {
		c.Mode = Mode(mode)
	}

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		c.OTELEndpoint = endpoint
	}

	if pid := os.Getenv("TARGET_PID"); pid != "" {
		c.TargetPID = pid
	}

	if script := os.Getenv("BPFTRACE_SCRIPT"); script != "" {
		c.BPFScript = script
	}

	if serviceName := os.Getenv("SERVICE_NAME"); serviceName != "" {
		c.ServiceName = serviceName
	}

	// Set mode-specific defaults
	switch c.Mode {
	case ModeNativeUSDT:
		if c.ServiceName == "bpftrace-exporter" {
			c.ServiceName = "usdt-native-exporter"
		}
		c.TracerName = "usdt-native-exporter"
	case ModeLibstabst:
		if c.ServiceName == "bpftrace-exporter" {
			c.ServiceName = "usdt-bpftrace-exporter"
		}
		c.TracerName = "bpftrace-exporter"
	}
}
