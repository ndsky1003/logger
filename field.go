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

func (this Field) Add(key string, value any) Field {
	this[key] = slog.AnyValue(value)
	return this
}

// 3 写死是没问题,因为外围是不会包装这个函数的,直接暴露Fields字段
func (this Field) Trace(msg ...any) {
	log(3, LevelTrace, this, msg...)
}

func (this Field) Tracef(msg string, args ...any) {
	logf(3, LevelTrace, this, msg, args...)
}

func (this Field) Debug(msg ...any) {
	log(3, LevelDebug, this, msg...)
}

func (this Field) Debugf(msg string, args ...any) {
	logf(3, LevelDebug, this, msg, args...)
}

func (this Field) Info(msg ...any) {
	log(3, LevelInfo, this, msg...)
}

func (this Field) Infof(msg string, args ...any) {
	logf(3, LevelInfo, this, msg, args...)
}

func (this Field) Notice(msg ...any) {
	log(3, LevelNotice, this, msg...)
}

func (this Field) Noticef(msg string, args ...any) {
	logf(3, LevelNotice, this, msg, args...)
}

func (this Field) Warn(msg ...any) {
	log(3, LevelWarn, this, msg...)
}

func (this Field) Warnf(msg string, args ...any) {
	logf(3, LevelWarn, this, msg, args...)
}

func (this Field) Err(msg ...any) {
	log(3, LevelErr, this, msg...)
}

func (this Field) Errf(msg string, args ...any) {
	logf(3, LevelErr, this, msg, args...)
}

func (this Field) Emergency(msg ...any) {
	log(3, LevelEmergency, this, msg...)
}

func (this Field) Emergencyf(msg string, args ...any) {
	logf(3, LevelEmergency, this, msg, args...)
}

func (this Field) Fatal(msg ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	log(3, LevelFatal, this, msg...)
	s := string(debug.Stack())
	log(3, LevelFatal, nil, s)
	fmt.Println(s)
}

func (this Field) Fatalf(msg string, args ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	logf(3, LevelFatal, this, msg, args...)
	s := string(debug.Stack())
	log(3, LevelFatal, nil, s)
	fmt.Println(s)
}
