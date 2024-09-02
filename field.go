package logger

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
)

type Field map[string]slog.Value

func Fields() Field {
	return Field{}
}

func get_local_skip() int {
	return get_skip() + 1
}

func (this Field) Add(key string, value any) Field {
	this[key] = slog.AnyValue(value)
	return this
}

func (this Field) Trace(msg ...any) {
	log_any(LevelTrace, this, msg...)
}

func (this Field) Tracef(msg string, args ...any) {
	logf(get_local_skip(), LevelTrace, this, msg, args...)
}

func (this Field) Debug(msg ...any) {
	log_any(LevelDebug, this, msg...)
}

func (this Field) Debugf(msg string, args ...any) {
	logf(get_local_skip(), LevelDebug, this, msg, args...)
}

func (this Field) Info(msg ...any) {
	log_any(LevelInfo, this, msg...)
}

func (this Field) Infof(msg string, args ...any) {
	logf(get_local_skip(), LevelInfo, this, msg, args...)
}

func (this Field) Notice(msg ...any) {
	log_any(LevelNotice, this, msg...)
}

func (this Field) Noticef(msg string, args ...any) {
	logf(get_local_skip(), LevelNotice, this, msg, args...)
}

func (this Field) Warn(msg ...any) {
	log_any(LevelWarn, this, msg...)
}

func (this Field) Warnf(msg string, args ...any) {
	logf(get_local_skip(), LevelWarn, this, msg, args...)
}

func (this Field) Err(msg ...any) {
	log_any(LevelErr, this, msg...)
}

func (this Field) Errf(msg string, args ...any) {
	logf(get_local_skip(), LevelErr, this, msg, args...)
}

func (this Field) Emergency(msg ...any) {
	log_any(LevelEmergency, this, msg...)
}

func (this Field) Emergencyf(msg string, args ...any) {
	logf(get_local_skip(), LevelEmergency, this, msg, args...)
}

func (this Field) Fatal(msg ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	log_any(LevelFatal, this, msg...)
	s := string(debug.Stack())
	log_any(LevelFatal, nil, s)
	fmt.Println(s)
}

func (this Field) Fatalf(msg string, args ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	logf(get_local_skip(), LevelFatal, this, msg, args...)
	s := string(debug.Stack())
	log_any(LevelFatal, nil, s)
	fmt.Println(s)
}
