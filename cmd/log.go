package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// LogWriter wraps a slog.Logger for io.Writer compatibility.
type LogWriter struct {
	logger *slog.Logger
	level  slog.Level
	ctx    context.Context
	offset int
	prefix string
}

type loggerKey struct{}

// New creates a new slog.Logger with tint handler.
func New(w *os.File) (*slog.Logger, *slog.LevelVar) {
	level := &slog.LevelVar{}
	level.Set(slog.LevelInfo)
	logOpt := &slog.HandlerOptions{Level: level}
	log := slog.New(tint.NewHandler(w, &tint.Options{
		Level:      logOpt.Level,
		TimeFormat: "15:04:05.000",
	}))
	return log, level
}

// WithLogger stores a logger in the context.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

// NewLogger creates a logger and cancellation function for the given context.
func NewLogger(ctx context.Context) (*slog.Logger, context.CancelCauseFunc) {
	log, _ := New(os.Stdout)
	ctx = WithLogger(ctx, log)
	ctx, cancel := context.WithCancelCause(ctx)
	return ctx.Value(loggerKey{}).(*slog.Logger), cancel
}

// NewLogWriter creates a LogWriter that writes to the given logger.
func NewLogWriter(ctx context.Context, logger *slog.Logger, level slog.Level, prefix string) *LogWriter {
	return &LogWriter{
		logger: logger,
		level:  level,
		ctx:    ctx,
		offset: 0,
		prefix: prefix,
	}
}
