package logger

import "log/slog"

func Options() *Option {
	return &Option{}
}

type Option struct {
	AddSource   *bool
	Level       slog.Leveler
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

func (o *Option) SetAddSource(addSource bool) *Option {
	if o == nil {
		return o
	}
	o.AddSource = &addSource
	return o
}

func (o *Option) SetLevel(level slog.Leveler) *Option {
	if o == nil {
		return o
	}
	o.Level = level
	return o
}

func (o *Option) SetReplaceAttr(replaceAttr func(groups []string, a slog.Attr) slog.Attr) *Option {
	if o == nil {
		return o
	}
	o.ReplaceAttr = replaceAttr
	return o
}

func (o *Option) merge(delta *Option) {
	if delta == nil || o == nil {
		return
	}
	if delta.AddSource != nil {
		o.AddSource = delta.AddSource
	}
	if delta.Level != nil {
		o.Level = delta.Level
	}
	if delta.ReplaceAttr != nil {
		o.ReplaceAttr = delta.ReplaceAttr
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
