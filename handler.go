// copy from offical
package logger

import (
	"context"
	"io"
	"log/slog"
	"runtime"
)

type custom_handler struct {
	opts              *slog.HandlerOptions
	preformattedAttrs []byte
	groupPrefix       string
	// groups            []string // all groups started from WithGroup
	// nOpenGroups       int      // the number of groups opened in preformattedAttrs
	w io.Writer
}

func NewCustomHandler(w io.Writer, opts *slog.HandlerOptions) *custom_handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &custom_handler{
		opts: opts,
		w:    w,
	}
}

// 没在这里控制
func (this *custom_handler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (this *custom_handler) WithAttrs(as []slog.Attr) slog.Handler {
	return this
}

func (this *custom_handler) WithGroup(name string) slog.Handler {
	return this
}

func (this *custom_handler) Handle(_ context.Context, r slog.Record) error {
	state := this.newHandleState(NewBuffer(), true, "")
	defer state.free()
	stateGroups := state.groups
	state.groups = nil // So ReplaceAttrs sees no groups instead of the pre groups.
	rep := this.opts.ReplaceAttr

	// level
	key := slog.LevelKey
	val := r.Level
	if rep == nil {
		state.appendKey(key)
		state.appendString(val.String())
	} else {
		state.appendAttr(slog.Any(key, val))
	}

	// time
	if !r.Time.IsZero() {
		key := slog.TimeKey
		val := r.Time.Round(0) // strip monotonic to match Attr behavior
		if rep == nil {
			state.appendKey(key)
			state.appendTime(val)
		} else {
			state.appendAttr(slog.Time(key, val))
		}
	}

	// 文件
	if this.opts.AddSource {
		state.appendAttr(slog.Any(slog.SourceKey, source(r.PC)))
	}

	key = slog.MessageKey
	msg := r.Message
	if rep == nil {
		state.appendKey(key)
		state.appendString(msg)
	} else {
		state.appendString(msg)
		// state.appendAttr(slog.String(key, msg))
		// state.appendString(msg)
	}
	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.
	state.appendNonBuiltIns(r)
	if err := state.buf.WriteByte('\n'); err != nil {
		return err
	}
	_, err := this.w.Write(*state.buf)
	return err
}

func (h *custom_handler) attrSep() string {
	return " "
}

// source returns a Source for the log event.
// If the Record was created without the necessary information,
// or if the location is unavailable, it returns a non-nil *Source
// with zero fields.
func source(r uintptr) *slog.Source {
	fs := runtime.CallersFrames([]uintptr{r})
	f, _ := fs.Next()
	return &slog.Source{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
}
