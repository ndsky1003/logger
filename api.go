package logger

import (
	"io"
	"log/slog"
	"os"
)

var programLevel = new(slog.LevelVar)

func SetProgramLevel(level slog.Level) {
	programLevel.Set(slog.LevelInfo)

}

func New(w io.Writer, opts ...*Option) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	opt := Options().SetLevel(programLevel).Merge(opts...)
	h := NewFastTextHandler(w, &opt)
	return slog.New(h)
}
