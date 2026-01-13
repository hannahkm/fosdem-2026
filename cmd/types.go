package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
)

type RunManyOpts struct {
	Logger   *slog.Logger
	Scenario []string
	Force    bool
	Num      int
}

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

type Request struct {
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error"`
}

type ProfilePayload struct {
	Error error    `json:"error"`
	Files []string `json:"files"`
	Bytes int64    `json:"bytes"`
}

type TracesPayload struct {
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
	Trace []byte `json:"trace"`
}

type LogPayload struct {
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
	Logs  []byte `json:"logs"`
}

type Client struct {
	*docker.Client
}

type BuildOpts struct {
	// Dir points to the directory containing the Dockerfile.
	Dir string
	// https://docs.docker.com/reference/cli/docker/buildx/build/#build-arg
	Args    map[string]string
	Secrets map[string]string
}

func NewClient(ctx context.Context) (*Client, error) {
	c, err := docker.NewClientWithOpts()
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

func (c *Client) BuildCommand(ctx context.Context, opts *BuildOpts) *exec.Cmd {
	args := []string{
		"build",
	}
	for k, v := range opts.Args {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	for id, env := range opts.Secrets {
		args = append(args, "--secret", fmt.Sprintf("id=%s,env=%s", id, env))
	}
	args = append(args, opts.Dir)
	return exec.CommandContext(ctx, "docker", args...)
}
