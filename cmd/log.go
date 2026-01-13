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

func NewLogger(ctx context.Context) *slog.Logger {
	return ctx.Value("logger").(*slog.Logger)
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
