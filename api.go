package logger

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
)

// api
// eg:美化每一个答应对象,json序列化等
func SetWrapFunc(f warp_func) {
	wrap_func_obj = f
}

func SetLevel(v slog.Level) {
	default_level = v
	Default().set_level(default_level)
}

func GetLevel() slog.Level {
	return default_level
}

func SetDeltaSkip(v int) {
	delta_skip = v
}

func SetFolder(f string) {
	if f == "" {
		return
	}
	default_folder = f
	if err := init_folder(default_folder); err != nil {
		panic(err)
	}
}

func SetCreateHandler(fn create_handler) {
	if is_set_daily {
		c.Remove(default_cron_id)
	}
	create_handler_func = fn
	initLogger(os.Stdout)
}

func SetDaily() {
	is_set_daily = true
	SetCreateHandler(DailyHandlerCreateFunc)
	init_daily()
}

func Close() {
	Default().close()
}

func Flush() {
	Close()
}

func Trace(msg ...any) {
	log_any(LevelTrace, nil, msg...)
}

func Tracef(msg string, args ...any) {
	logf(get_skip(), LevelTrace, nil, msg, args...)
}

func Debug(msg ...any) {
	log_any(LevelDebug, nil, msg...)
}

func Debugf(msg string, args ...any) {
	logf(get_skip(), LevelDebug, nil, msg, args...)
}

func Info(msg ...any) {
	log_any(LevelInfo, nil, msg...)
}

func Infof(msg string, args ...any) {
	logf(get_skip(), LevelInfo, nil, msg, args...)
}

func Notice(msg ...any) {
	log_any(LevelNotice, nil, msg...)
}

func Noticef(msg string, args ...any) {
	logf(get_skip(), LevelNotice, nil, msg, args...)
}

func Warn(msg ...any) {
	log_any(LevelWarn, nil, msg...)
}

func Warnf(msg string, args ...any) {
	logf(get_skip(), LevelWarn, nil, msg, args...)
}

func Err(msg ...any) {
	log_any(LevelErr, nil, msg...)
}

func Errf(msg string, args ...any) {
	logf(get_skip(), LevelErr, nil, msg, args...)
}

func Emergency(msg ...any) {
	log_any(LevelEmergency, nil, msg...)
}

func Emergencyf(msg string, args ...any) {
	logf(get_skip(), LevelEmergency, nil, msg, args...)
}

func Fatalf(format string, v ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	logf(get_skip(), LevelFatal, nil, format, v...)
	s := string(debug.Stack())
	log_any(LevelFatal, nil, s)
	fmt.Println(s)
}

func Fatalln(v ...any) {
	defer func() {
		Flush()
		os.Exit(1)
	}()
	log_any(LevelFatal, nil, v...)
	s := string(debug.Stack())
	log_any(LevelFatal, nil, s)
	fmt.Println(s)
}
