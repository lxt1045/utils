package log

import (
	"log/slog"

	eslog "github.com/lxt1045/errors/slog"
)

func NewHandler(h slog.Handler) slog.Handler {
	return eslog.NewHandler(h)
}

func NewSlog(h slog.Handler) *slog.Logger {
	return eslog.New(h)
}
