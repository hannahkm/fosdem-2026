// Package main provides the CLI entry point for the FOSDEM 2026 experiment runner.
package main

import (
	"context"
	"fmt"
	"os"

	"fosdem2026/cmd"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "fosdem",
		Usage: "FOSDEM 2026 experiment runner",
		Commands: []*cli.Command{
			cmd.CmdRun,
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
