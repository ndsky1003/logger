package logger

import (
	"log/slog"
)

func New(opts ...*Option) *slog.Logger {
	h := NewFastTextHandler(opts...)
	return slog.New(h)
}

func SetDefault(opts ...*Option) {
	slog.SetDefault(New(opts...))
}
