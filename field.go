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

func (this field) Trace(msg any) {
	log_any(LevelTrace, this, msg)
}

func (this field) Tracef(msg string, args ...any) {
	logf(3, LevelTrace, this, msg, args...)
}

func (this field) Debug(msg any) {
	log_any(LevelDebug, this, msg)
}

func (this field) Debugf(msg string, args ...any) {
	logf(3, LevelDebug, this, msg, args...)
}

func (this field) Info(msg any) {
	log_any(LevelInfo, this, msg)
}

func (this field) Infof(msg string, args ...any) {
	logf(3, LevelInfo, this, msg, args...)
}

func (this field) Notice(msg any) {
	log_any(LevelNotice, this, msg)
}

func (this field) Noticef(msg string, args ...any) {
	logf(3, LevelNotice, this, msg, args...)
}

func (this field) Warn(msg any) {
	log_any(LevelWarn, this, msg)
}

func (this field) Warnf(msg string, args ...any) {
	logf(3, LevelWarn, this, msg, args...)
}

func (this field) Err(msg any) {
	log_any(LevelErr, this, msg)
}

func (this field) Errf(msg string, args ...any) {
	logf(3, LevelErr, this, msg, args...)
}

func (this field) Emergency(msg any) {
	log_any(LevelEmergency, this, msg)
}

func (this field) Emergencyf(msg string, args ...any) {
	logf(3, LevelEmergency, this, msg, args...)
}
