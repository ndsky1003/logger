package logger

import (
	"context"
	"log/slog"
)

type default_handler struct {
	handler slog.Handler
}

func new_default_handler(h slog.Handler) *default_handler {
	return &default_handler{
		handler: h,
	}
}

func (this *default_handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= programLevel.Level()
}

func (this *default_handler) Handle(ctx context.Context, r slog.Record) error {
	return this.handler.Handle(ctx, r)
}

func (this *default_handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return new_default_handler(this.handler.WithAttrs(attrs))
}

func (this *default_handler) WithGroup(name string) slog.Handler {
	return new_default_handler(this.handler.WithGroup(name))
}
