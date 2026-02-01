package cmd

import (
	"context"
	"time"

	"github.com/goccy/go-json"
	"github.com/urfave/cli/v3"
)

// CmdRun is the CLI command for running experiments.
var CmdRun = &cli.Command{
	Name:    "run",
	Aliases: []string{"r"},
	Usage:   "runs one or more experiments",
	Description: `
	Run one or more experiments.

	Available scenarios: default (no instrumentation), manual (manual instrumentation using OTel SDK), obi (Opentelemetry eBPF Instrumentation),
	ebpf (OpenTelemetry "Auto Instrumentation"), orchestrion (compile-time instrumentation using Orchestrion), injector (Frida dynamic instrumentation),
	libstabst (USDT probes with salp library), usdt (native USDT probes via Go fork), flightrecorder (Flight Recorder distributed tracing via Go fork)

	Use scenario "all" to run all scenarios.

	Run with "stop" to clean up the environment.
	`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Do not use cached outputs, rerun everything.",
		},
		&cli.IntFlag{
			Name:    "num",
			Aliases: []string{"n"},
			Usage:   "The number of times to repeat each experiment.",
			Value:   1,
		},
		&cli.StringFlag{
			Name:  "scenario",
			Usage: "The scenario to run",
			Value: "default",
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Usage:   "Timeout for each test run (e.g., 5m, 10m, 1h)",
			Value:   5 * time.Minute,
		},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		log, cancel := NewLogger(ctx)
		defer cancel(nil)
		opts := RunManyOpts{
			Logger:   log,
			Scenario: []string{c.String("scenario")},
			Num:      c.Int("num"),
			Force:    c.Bool("force"),
			Timeout:  c.Duration("timeout"),
			Inputs: &Input{
				Port:           8080,
				RuntimeVersion: "1.25.5",
				Flush:          true,
				RPS:            1,
				Duration:       30,
				Timeout:        5,
			},
		}

		results, err := Many(ctx, &opts)
		if err != nil {
			return err
		}

		_, _ = c.Writer.Write([]byte("[\n"))
		for i, result := range results {
			if i > 0 {
				_, _ = c.Writer.Write([]byte(",\n"))
			}
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return err
			}
			_, _ = c.Writer.Write([]byte("  "))
			_, _ = c.Writer.Write(resultJSON)
		}
		_, _ = c.Writer.Write([]byte("\n]\n"))
		return nil
	},
}
