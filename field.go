package logger

import "log/slog"

type field map[string]slog.Value

func Fields() field {
	return field{}
}

func (this field) Add(key string, value slog.Value) field {
	this[key] = value
	return this
}

func (this field) Trace(msg string) {
	log(LevelTrace, this, msg)
}

func (this field) Tracef(msg string, args ...any) {
	logf(LevelTrace, this, msg, args...)
}

func (this field) Debug(msg string) {
	log(LevelDebug, this, msg)
}

func (this field) Debugf(msg string, args ...any) {
	logf(LevelDebug, this, msg, args...)
}

func (this field) Info(msg string) {
	log(LevelInfo, this, msg)
}

func (this field) Infof(msg string, args ...any) {
	logf(LevelInfo, this, msg, args...)
}

func (this field) Notice(msg string) {
	log(LevelNotice, this, msg)
}

func (this field) Noticef(msg string, args ...any) {
	logf(LevelNotice, this, msg, args...)
}

func (this field) Warn(msg string) {
	log(LevelWarn, this, msg)
}

func (this field) Warnf(msg string, args ...any) {
	logf(LevelWarn, this, msg, args...)
}

func (this field) Err(msg string) {
	log(LevelErr, this, msg)
}

func (this field) Errf(msg string, args ...any) {
	logf(LevelErr, this, msg, args...)
}

func (this field) Emergency(msg string) {
	log(LevelEmergency, this, msg)
}

func (this field) Emergencyf(msg string, args ...any) {
	logf(LevelEmergency, this, msg, args...)
}
