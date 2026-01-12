package cmd

import (
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
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
