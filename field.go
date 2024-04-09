package logger

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
)

type field map[string]slog.Value

func Fields() field {
	return field{}
}

func (this field) Add(key string, value any) field {
	this[key] = slog.AnyValue(value)
	return this
}

func (this field) Trace(msg ...any) {
	log_any(LevelTrace, this, msg...)
}

func (this field) Tracef(msg string, args ...any) {
	logf(3, LevelTrace, this, msg, args...)
}

func (this field) Debug(msg ...any) {
	log_any(LevelDebug, this, msg...)
}

func (this field) Debugf(msg string, args ...any) {
	logf(3, LevelDebug, this, msg, args...)
}

func (this field) Info(msg ...any) {
	log_any(LevelInfo, this, msg...)
}

func (this field) Infof(msg string, args ...any) {
	logf(3, LevelInfo, this, msg, args...)
}

func (this field) Notice(msg ...any) {
	log_any(LevelNotice, this, msg...)
}

func (this field) Noticef(msg string, args ...any) {
	logf(3, LevelNotice, this, msg, args...)
}

func (this field) Warn(msg ...any) {
	log_any(LevelWarn, this, msg...)
}

func (this field) Warnf(msg string, args ...any) {
	logf(3, LevelWarn, this, msg, args...)
}

func (this field) Err(msg ...any) {
	log_any(LevelErr, this, msg...)
}

func (this field) Errf(msg string, args ...any) {
	logf(3, LevelErr, this, msg, args...)
}

func (this field) Emergency(msg ...any) {
	log_any(LevelEmergency, this, msg...)
}

func (this field) Emergencyf(msg string, args ...any) {
	logf(3, LevelEmergency, this, msg, args...)
}

func (this field) Fatal(msg ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	log_any(LevelFatal, this, msg...)
	s := string(debug.Stack())
	log_any(LevelFatal, nil, s)
	fmt.Println(s)
}

func (this field) Fatalf(msg string, args ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	logf(3, LevelFatal, this, msg, args...)
	s := string(debug.Stack())
	log_any(LevelFatal, nil, s)
	fmt.Println(s)
}
