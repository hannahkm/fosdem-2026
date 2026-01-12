package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

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
