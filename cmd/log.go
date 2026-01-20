package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/lmittmann/tint"
)

type LogWriter struct {
	mu     sync.Mutex
	logger *slog.Logger
	level  slog.Level
	buf    bytes.Buffer
	ctx    context.Context
	offset int
	prefix string
}

type loggerKey struct{}

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

func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

func NewLogger(ctx context.Context) (*slog.Logger, context.CancelCauseFunc) {
	log, _ := New(os.Stdout)
	ctx = WithLogger(ctx, log)
	ctx, cancel := context.WithCancelCause(ctx)
	return ctx.Value(loggerKey{}).(*slog.Logger), cancel
}

func NewLogWriter(ctx context.Context, logger *slog.Logger, level slog.Level, prefix string) *LogWriter {
	return &LogWriter{
		logger: logger,
		level:  level,
		ctx:    ctx,
		offset: 0,
		prefix: prefix,
	}
}
