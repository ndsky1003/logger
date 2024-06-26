package logger

import "log/slog"

// 这里没有位运算
const (
	LevelTrace     = slog.Level(-8)
	LevelDebug     = slog.LevelDebug
	LevelInfo      = slog.LevelInfo
	LevelNotice    = slog.Level(2)
	LevelWarn      = slog.LevelWarn
	LevelErr       = slog.LevelError
	LevelEmergency = slog.Level(12)
	LevelFatal     = slog.Level(16)
)

const default_skip = 3

// 外层包装的需要动态设置这个,才能断点到行
var delta_skip = 0
