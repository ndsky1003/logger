package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	exe           string
	programLevel  = new(slog.LevelVar)
	defaultLogger atomic.Value
)

func init() {
	exe = path.Base(os.Args[0])
	SetLevel(LevelInfo)
	c := cron.New(cron.WithSeconds())
	c.AddFunc("0 0 0 * * *", func() {
		now := time.Now().Add(1 * time.Minute)
		yesterdayLogger := Default()
		if yesterdayLogger != nil {
			if err := yesterdayLogger.Close(); err != nil {
				fmt.Printf("err:%v,stack:%s", err, debug.Stack())
			}
		}
		defaultLogger.CompareAndSwap(yesterdayLogger, newLogger(now))
	})
	c.Start()

	if l := Default(); l == nil {
		defaultLogger.CompareAndSwap(nil, newLogger(time.Now()))
	}
}

func Default() *logger {
	if v := defaultLogger.Load(); v != nil {
		return v.(*logger)
	} else {
		return nil
	}
}

type logger struct {
	wc     io.WriteCloser
	logger *slog.Logger
}

func (this *logger) Close() error {
	if this == nil {
		return nil
	}
	return this.wc.Close()
}

func newLogger(now time.Time) *logger {
	filename := fmt.Sprintf("%s-%v.log", exe, now.Format(time.DateOnly))
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
	h := slog.NewTextHandler(f, opt)
	return &logger{
		wc:     f,
		logger: slog.New(h),
	}
}

func SetLevel(v slog.Level) {
	programLevel.Set(v)
}

var (
	cache = map[string]string{}
	opt   = &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "t"
				a.Value = slog.StringValue(a.Value.Any().(time.Time).Format("01-02 15:04:05"))
			}
			if a.Key == slog.LevelKey {
				a.Key = "l"
				level := a.Value.Any().(slog.Level)
				switch {
				case level < LevelDebug:
					a.Value = slog.StringValue("TRACE")
				case level < LevelInfo:
					a.Value = slog.StringValue("DEBUG")
				case level < LevelNotice:
					a.Value = slog.StringValue("INFO")
				case level < LevelWarning:
					a.Value = slog.StringValue("NOTICE")
				case level < LevelError:
					a.Value = slog.StringValue("WARNING")
				case level < LevelEmergency:
					a.Value = slog.StringValue("ERROR")
				default:
					a.Value = slog.StringValue("EMERGENCY")
				}
			}
			if a.Key == slog.SourceKey {
				a.Key = "s"
				source := a.Value.Any().(*slog.Source)
				if v, ok := cache[source.File]; ok {
					source.File = v
				} else {
					files := strings.Split(source.File, string(filepath.Separator))
					length := len(files)
					realPath := source.File
					if length > 3 {
						files = files[length-3:]
						realPath = filepath.Join(files...)
					}
					cache[source.File] = realPath
					source.File = realPath

				}

			}
			return a
		},
	}
)

func (this *logger) Infof(format string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:]) // skip [Callers, Infof]
	r := slog.NewRecord(time.Now(), slog.LevelInfo, fmt.Sprintf(format, args...), pcs[0])
	r.Add("age", 18)
	_ = this.logger.Handler().Handle(context.Background(), r)
}

func Info(v string) {
	if l := Default(); l != nil {
		l.Infof("name:%v", "liyang")
	} else {
		panic("dd")
	}
}
