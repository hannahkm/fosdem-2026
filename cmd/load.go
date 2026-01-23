package cmd

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type Config struct {
	Client      *http.Client
	Log         *slog.Logger
	URL         string
	RPS         int
	Clients     int
	Duration    float64
	Endpoints   int
	ExpectError bool
}

func Generate(ctx context.Context, config *Config) (requests []Request, err error) {
	if config.Clients > 0 && config.RPS > 0 {
		return nil, fmt.Errorf("clients and rps cannot be set at the same time")
	}

	var mu sync.Mutex
	requestFn := func(ctx context.Context, i int) error {
		start := time.Now()
		url := fmt.Sprintf("%s", config.URL)
		req := Request{}
		err := doRequest(ctx, config.Client, url, config.ExpectError)
		if err != nil {
			req.Error = err.Error()
		}
		req.End = time.Now()
		req.Duration = req.End.Sub(start)
		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()
		return err
	}

	duration := time.Duration(config.Duration * 1e9)
	if config.Clients > 0 {
		config.Log.Info("⌛ load starting (closed loop)", "clients", config.Clients, "duration", config.Duration, "url", config.URL)
		err = ClosedLoop(ctx, config.Clients, duration, requestFn)
	} else {
		config.Log.Info("⌛ load starting (open loop)", "requests", int(float64(config.RPS)*config.Duration), "duration", config.Duration, "url", config.URL)
		err = OpenLoop(ctx, config.RPS, duration, requestFn)
	}
	if err != nil {
		config.Log.Error("load done", "requests", len(requests), "errors", countErrors(requests), "first_error", err)
	} else {
		config.Log.Info("✅ load done", "requests", len(requests), "errors", 0)
	}
	return requests, err
}

func doRequest(ctx context.Context, client *http.Client, url string, expectError bool) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if expectError {
		if resp.StatusCode < 500 {
			return fmt.Errorf("expected error response: status=%d body=%s", resp.StatusCode, string(data))
		}
		return nil
	}
	want := []byte("Hello World\n")
	if !bytes.Equal(data, want) {
		return fmt.Errorf("invalid response: got=%s, want=%s", string(data), string(want))
	}
	return nil

}

func countErrors(requests []Request) int {
	var errors int
	for _, request := range requests {
		if request.Error != "" {
			errors++
		}
	}
	return errors
}

// OpenLoop schedules fn calls at a fixed rate (rps) for the given duration.
// Each call runs in its own goroutine, so slow calls don't affect the schedule.
// Context cancellation stops scheduling new calls but does not cancel in-flight ones.
func OpenLoop(ctx context.Context, rps int, duration time.Duration, fn func(context.Context, int) error) error {
	n := int(float64(rps) * duration.Seconds())
	var eg errgroup.Group
	var ctxErr error
	start := time.Now()
loop:
	for i := range n {
		select {
		case <-ctx.Done():
			ctxErr = ctx.Err()
			break loop
		case <-time.After(time.Until(start.Add(time.Duration(i+1) * (time.Second / time.Duration(rps))))):
		}
		eg.Go(func() error {
			return fn(ctx, i)
		})
	}
	return cmp.Or(eg.Wait(), ctxErr)
}

// ClosedLoop runs fn sequentially in each of the worker goroutines for the given duration.
// The rate is determined by how fast fn completes (closed-loop control).
// Once the duration elapses, the context passed to fn is cancelled.
func ClosedLoop(ctx context.Context, workers int, duration time.Duration, fn func(context.Context, int) error) error {
	var eg errgroup.Group
	deadline := time.Now().Add(duration)
	deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	var i atomic.Int64
	for range workers {
		eg.Go(func() error {
			var firstErr error
			for {
				select {
				case <-deadlineCtx.Done():
					return firstErr
				default:
				}
				firstErr = cmp.Or(firstErr, fn(deadlineCtx, int(i.Add(1)-1)))
			}
		})
	}
	return eg.Wait()
}
