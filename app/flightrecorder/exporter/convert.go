package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/trace"
)

// spanKey uniquely identifies a span for matching begin/end events
type spanKey struct {
	goroutine trace.GoID
	name      string
	kind      string // "range", "task", or "region"
}

// activeSpan tracks an in-progress span
type activeSpan struct {
	span      oteltrace.Span
	ctx       context.Context
	startTime time.Time
	traceTime trace.Time
	taskID    trace.TaskID // For task spans
}

// convertTraceToSpans reads FlightRecorder trace events and converts them to OTLP spans.
//
// Event type mappings:
// - EventTaskBegin/End -> Root spans (runtime/trace.NewTask)
// - EventRegionBegin/End -> Child spans (runtime/trace.StartRegion)
// - EventRangeBegin/End -> Internal spans (GC, network, etc.)
// - EventLog -> Span events (runtime/trace.Log)
// - EventStateTransition -> Span events (goroutine/proc state changes)
func convertTraceToSpans(ctx context.Context, r io.Reader) error {
	reader, err := trace.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create trace reader: %w", err)
	}

	tracer := otel.Tracer("flightrecorder")
	activeSpans := make(map[spanKey]*activeSpan)
	taskSpans := make(map[trace.TaskID]*activeSpan) // Index tasks by TaskID for parent lookups

	// Use current time as reference for converting trace timestamps
	baseWallTime := time.Now()
	var baseTraceTime trace.Time
	baseTimeSet := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ev, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading event: %w", err)
		}

		// Set base trace time from first event
		if !baseTimeSet {
			baseTraceTime = ev.Time()
			baseTimeSet = true
		}

		// Convert trace events to spans based on event kind
		switch ev.Kind() {
		// Range events - generic runtime instrumentation
		case trace.EventRangeBegin:
			handleRangeBegin(ctx, tracer, ev, baseWallTime, baseTraceTime, activeSpans)
		case trace.EventRangeEnd:
			handleRangeEnd(ev, baseWallTime, baseTraceTime, activeSpans)

		// Task events - similar to distributed trace spans (from runtime/trace.NewTask)
		case trace.EventTaskBegin:
			handleTaskBegin(ctx, tracer, ev, baseWallTime, baseTraceTime, activeSpans, taskSpans)
		case trace.EventTaskEnd:
			handleTaskEnd(ev, baseWallTime, baseTraceTime, activeSpans, taskSpans)

		// Region events - synchronous span-like regions (from runtime/trace.StartRegion)
		case trace.EventRegionBegin:
			handleRegionBegin(ctx, tracer, ev, baseWallTime, baseTraceTime, activeSpans, taskSpans)
		case trace.EventRegionEnd:
			handleRegionEnd(ev, baseWallTime, baseTraceTime, activeSpans)

		// Log events - converted to span events
		case trace.EventLog:
			handleLogEvent(ev, baseWallTime, baseTraceTime, activeSpans, taskSpans)

		// State transitions - goroutine lifecycle events
		case trace.EventStateTransition:
			handleStateTransition(ev, baseWallTime, baseTraceTime, activeSpans)
		}
	}

	// End any remaining active spans (shouldn't happen with well-formed traces)
	for key, as := range activeSpans {
		as.span.SetStatus(codes.Error, "span not properly closed")
		as.span.End()
		delete(activeSpans, key)
	}

	return nil
}

func handleRangeBegin(ctx context.Context, tracer oteltrace.Tracer, ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan) {
	r := ev.Range()
	key := spanKey{goroutine: ev.Goroutine(), name: r.Name, kind: "range"}

	wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)
	spanCtx, span := tracer.Start(ctx, r.Name,
		oteltrace.WithTimestamp(wallTime),
		oteltrace.WithAttributes(
			attribute.Int64("goroutine.id", int64(ev.Goroutine())),
			attribute.String("trace.event.kind", "range"),
			attribute.String("range.scope.kind", r.Scope.Kind.String()),
		),
	)

	activeSpans[key] = &activeSpan{
		span:      span,
		ctx:       spanCtx,
		startTime: wallTime,
		traceTime: ev.Time(),
	}

	// Add stack trace if available
	addStackAttributes(span, ev)
}

func handleRangeEnd(ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan) {
	r := ev.Range()
	key := spanKey{goroutine: ev.Goroutine(), name: r.Name, kind: "range"}

	if as, ok := activeSpans[key]; ok {
		wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)
		duration := wallTime.Sub(as.startTime)
		as.span.SetAttributes(
			attribute.Int64("duration_ns", duration.Nanoseconds()),
			attribute.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		)
		as.span.End(oteltrace.WithTimestamp(wallTime))
		delete(activeSpans, key)
	}
}

func handleTaskBegin(ctx context.Context, tracer oteltrace.Tracer, ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan, taskSpans map[trace.TaskID]*activeSpan) {
	task := ev.Task()
	key := spanKey{goroutine: ev.Goroutine(), name: task.Type, kind: "task"}

	wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)

	// Check if this task has a parent task
	parentCtx := ctx
	if task.Parent != 0 {
		if parentAS, ok := taskSpans[task.Parent]; ok {
			parentCtx = parentAS.ctx
		}
	}

	spanCtx, span := tracer.Start(parentCtx, task.Type,
		oteltrace.WithTimestamp(wallTime),
		oteltrace.WithAttributes(
			attribute.Int64("goroutine.id", int64(ev.Goroutine())),
			attribute.String("trace.event.kind", "task"),
			attribute.Int64("task.id", int64(task.ID)),
		),
	)

	if task.Parent != 0 {
		span.SetAttributes(attribute.Int64("task.parent_id", int64(task.Parent)))
	}

	as := &activeSpan{
		span:      span,
		ctx:       spanCtx,
		startTime: wallTime,
		traceTime: ev.Time(),
		taskID:    task.ID,
	}

	activeSpans[key] = as
	taskSpans[task.ID] = as

	addStackAttributes(span, ev)
}

func handleTaskEnd(ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan, taskSpans map[trace.TaskID]*activeSpan) {
	task := ev.Task()
	key := spanKey{goroutine: ev.Goroutine(), name: task.Type, kind: "task"}

	if as, ok := activeSpans[key]; ok {
		wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)
		duration := wallTime.Sub(as.startTime)
		as.span.SetAttributes(
			attribute.Int64("duration_ns", duration.Nanoseconds()),
			attribute.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		)
		as.span.End(oteltrace.WithTimestamp(wallTime))
		delete(activeSpans, key)
		delete(taskSpans, task.ID)
	}
}

func handleRegionBegin(ctx context.Context, tracer oteltrace.Tracer, ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan, taskSpans map[trace.TaskID]*activeSpan) {
	region := ev.Region()
	key := spanKey{goroutine: ev.Goroutine(), name: region.Type, kind: "region"}

	wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)

	// Regions are children of tasks
	parentCtx := ctx
	if region.Task != 0 {
		if taskAS, ok := taskSpans[region.Task]; ok {
			parentCtx = taskAS.ctx
		}
	}

	spanCtx, span := tracer.Start(parentCtx, region.Type,
		oteltrace.WithTimestamp(wallTime),
		oteltrace.WithAttributes(
			attribute.Int64("goroutine.id", int64(ev.Goroutine())),
			attribute.String("trace.event.kind", "region"),
		),
	)

	if region.Task != 0 {
		span.SetAttributes(attribute.Int64("region.task_id", int64(region.Task)))
	}

	activeSpans[key] = &activeSpan{
		span:      span,
		ctx:       spanCtx,
		startTime: wallTime,
		traceTime: ev.Time(),
		taskID:    region.Task,
	}

	addStackAttributes(span, ev)
}

func handleRegionEnd(ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan) {
	region := ev.Region()
	key := spanKey{goroutine: ev.Goroutine(), name: region.Type, kind: "region"}

	if as, ok := activeSpans[key]; ok {
		wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)
		duration := wallTime.Sub(as.startTime)
		as.span.SetAttributes(
			attribute.Int64("duration_ns", duration.Nanoseconds()),
			attribute.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		)
		as.span.End(oteltrace.WithTimestamp(wallTime))
		delete(activeSpans, key)
	}
}

func handleLogEvent(ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan, taskSpans map[trace.TaskID]*activeSpan) {
	logData := ev.Log()
	wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)

	// Try to find the task span this log belongs to
	if logData.Task != 0 {
		if taskAS, ok := taskSpans[logData.Task]; ok {
			taskAS.span.AddEvent(logData.Message,
				oteltrace.WithTimestamp(wallTime),
				oteltrace.WithAttributes(
					attribute.String("log.category", logData.Category),
				),
			)
			return
		}
	}

	// Fall back to finding any active span on this goroutine
	for key, as := range activeSpans {
		if key.goroutine == ev.Goroutine() {
			as.span.AddEvent(logData.Message,
				oteltrace.WithTimestamp(wallTime),
				oteltrace.WithAttributes(
					attribute.String("log.category", logData.Category),
				),
			)
			return
		}
	}
}

func handleStateTransition(ev trace.Event, baseWallTime time.Time, baseTraceTime trace.Time, activeSpans map[spanKey]*activeSpan) {
	st := ev.StateTransition()
	wallTime := eventWallTime(ev.Time(), baseWallTime, baseTraceTime)

	// Find active span for this goroutine and add state transition as event
	for key, as := range activeSpans {
		if key.goroutine == ev.Goroutine() {
			// Determine resource type for clearer event names
			eventName := "state_transition"
			var attrs []attribute.KeyValue

			switch st.Resource.Kind {
			case trace.ResourceGoroutine:
				eventName = "goroutine_state"
				from, to := st.Goroutine()
				attrs = []attribute.KeyValue{
					attribute.String("from_state", from.String()),
					attribute.String("to_state", to.String()),
					attribute.Int64("resource.goroutine_id", int64(st.Resource.Goroutine())),
				}
			case trace.ResourceProc:
				eventName = "proc_state"
				from, to := st.Proc()
				attrs = []attribute.KeyValue{
					attribute.String("from_state", from.String()),
					attribute.String("to_state", to.String()),
					attribute.Int64("resource.proc_id", int64(st.Resource.Proc())),
				}
			default:
				attrs = []attribute.KeyValue{
					attribute.String("resource.kind", st.Resource.Kind.String()),
					attribute.String("reason", st.Reason),
				}
			}

			as.span.AddEvent(eventName, oteltrace.WithTimestamp(wallTime), oteltrace.WithAttributes(attrs...))
			return
		}
	}
}

// addStackAttributes adds stack trace information as span attributes.
func addStackAttributes(span oteltrace.Span, ev trace.Event) {
	stack := ev.Stack()
	if stack == trace.NoStack {
		return
	}

	var frames []string
	var functions []string
	frameCount := 0
	maxFrames := 10

	stack.Frames(func(f trace.StackFrame) bool {
		if frameCount >= maxFrames {
			return false
		}
		frames = append(frames, f.File+":"+strconv.FormatUint(f.Line, 10))
		functions = append(functions, f.Func)
		frameCount++
		return true
	})

	if len(frames) > 0 {
		span.SetAttributes(
			attribute.StringSlice("code.stacktrace.frames", frames),
			attribute.StringSlice("code.stacktrace.functions", functions),
		)
	}
}

// eventWallTime converts a trace timestamp to wall clock time.
func eventWallTime(eventTime trace.Time, baseWallTime time.Time, baseTraceTime trace.Time) time.Time {
	offset := time.Duration(int64(eventTime) - int64(baseTraceTime))
	return baseWallTime.Add(offset)
}
