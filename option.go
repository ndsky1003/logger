package logger

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

func Options() *Option {
	return &Option{}
}

type Option struct {
	w            io.Writer
	level        *Level
	addSource    *bool
	replaceAttr  func(groups []string, a slog.Attr) slog.Attr
	extractorfn  func(ctx context.Context) []slog.Attr //提取context的元数据,设置到record的attr上
	forcedebugfn func(ctx context.Context) bool        //强制打印的信息
}

func (o *Option) SetExtractorAttr(fn func(context.Context) []slog.Attr) *Option {
	if o == nil {
		return o
	}
	o.extractorfn = fn
	return o
}

func (o *Option) SetAddSource(addSource bool) *Option {
	if o == nil {
		return o
	}
	o.addSource = &addSource
	return o
}

func (o *Option) SetWriter(w io.Writer) *Option {
	if o == nil {
		return o
	}
	o.w = w
	return o
}

func (o *Option) SetLevel(level Level) *Option {
	if o == nil {
		return o
	}
	o.level = &level
	return o
}

func (o *Option) SetLevelString(lvl string) *Option {
	if o == nil {
		return o
	}
	var level slog.Level
	switch strings.ToLower(lvl) {
	case "debug":
		level = LevelDebug
	case "info":
		level = LevelInfo
	case "warn":
		level = LevelWarn
	case "error":
		level = LevelError
	default:
		level = LevelInfo
	}
	return o.SetLevel(level)
}

func (o *Option) SetReplaceAttr(replaceAttr func(groups []string, a slog.Attr) slog.Attr) *Option {
	if o == nil {
		return o
	}
	o.replaceAttr = replaceAttr
	return o
}

func (o *Option) merge(delta *Option) {
	if delta == nil || o == nil {
		return
	}
	if delta.addSource != nil {
		o.addSource = delta.addSource
	}
	if delta.level != nil {
		o.level = delta.level
	}
	if delta.replaceAttr != nil {
		o.replaceAttr = delta.replaceAttr
	}
	if delta.w != nil {
		o.w = delta.w
	}

	if delta.extractorfn != nil {
		o.extractorfn = delta.extractorfn
	}
}

func (o Option) Merge(opts ...*Option) Option {
	if len(opts) == 0 {
		return o
	}
	for _, delta := range opts {
		o.merge(delta)
	}
	return o
}
