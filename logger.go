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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

const record_length = 128

var (
	exe           string
	defaultLogger atomic.Value
	lock          sync.Mutex
)

func init() {
	exe = path.Base(os.Args[0])
	SetLevel(LevelInfo)
	c := cron.New(cron.WithSeconds())
	c.AddFunc("0 0 0 * * *", func() {
		now := time.Now().Add(1 * time.Minute)
		initLogger(now)
	})
	c.Start()
	initLogger(time.Now())
}

func Default() *logger {
	if v := defaultLogger.Load(); v != nil {
		return v.(*logger)
	} else {
		return nil
	}
}

type logger struct {
	wc         io.WriteCloser
	wg         *sync.WaitGroup
	logger     *slog.Logger
	level      slog.LevelVar // 打印的等级优先级
	chanRecord chan *slog.Record
}

func (this *logger) close() {
	if this == nil {
		return
	}
	this.wg.Wait()
	if err := this.wc.Close(); err != nil {
		fmt.Println("===err:", err)
	}
}

func (this *logger) Close() {
	if this != nil {
		go this.close()
	}
}

func initLogger(now time.Time) {
	lock.Lock()
	defer lock.Unlock()
	oldLogger := Default()
	oldLogger.Close()
	tmpLogger := newLogger(now)
	if oldLogger == nil {
		if b := defaultLogger.CompareAndSwap(nil, tmpLogger); b {
			slog.SetDefault(tmpLogger.logger)
		}
	} else {
		if b := defaultLogger.CompareAndSwap(oldLogger, tmpLogger); b {
			slog.SetDefault(tmpLogger.logger)
		}
	}
}

func newLogger(now time.Time) *logger {
	filename := fmt.Sprintf("%s-%v.log", exe, now.Format(time.DateOnly))
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	h := slog.NewTextHandler(f, opt)
	l := &logger{
		wc:         f,
		wg:         &sync.WaitGroup{},
		logger:     slog.New(h),
		chanRecord: make(chan *slog.Record, record_length),
	}
	go l._log()
	return l
}

func (this *logger) log(r *slog.Record) {
	if this == nil {
		fmt.Printf("discall log:%+v\n", r)
		return
	}
	if r == nil {
		return
	}
	if r.Level < this.level.Level() {
		return
	}
	this.wg.Add(1)
	this.chanRecord <- r
}

func (this *logger) _log() {
	for v := range this.chanRecord {
		this.logger.Handler().Handle(context.Background(), *v)
		this.wg.Done()
	}
}

func (this *logger) set_level(v slog.Level) {
	if this == nil {
		return
	}
	this.level.Set(v)
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
				case level < LevelWarn:
					a.Value = slog.StringValue("NOTICE")
				case level < LevelErr:
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

func logf(level slog.Level, f field, msg string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, fmt.Sprintf(msg, args...), pcs[0])
	if f != nil {
		attrs := make([]slog.Attr, 0, len(f))
		for k, v := range f {
			attrs = append(attrs, slog.Attr{Key: k, Value: v})
		}
		r.AddAttrs(attrs...)
	}
	Default().log(&r)
}

func log(level slog.Level, f field, msg string) {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	if f != nil {
		attrs := make([]slog.Attr, 0, len(f))
		for k, v := range f {
			attrs = append(attrs, slog.Attr{Key: k, Value: v})
		}
		r.AddAttrs(attrs...)
	}
	Default().log(&r)
}

// api
func SetLevel(v slog.Level) {
	Default().set_level(v)
}

func Trace(msg string) {
	log(LevelTrace, nil, msg)
}

func Tracef(msg string, args ...any) {
	logf(LevelTrace, nil, msg, args...)
}

func Debug(msg string) {
	log(LevelDebug, nil, msg)
}

func Debugf(msg string, args ...any) {
	logf(LevelDebug, nil, msg, args...)
}

func Info(msg string) {
	log(LevelInfo, nil, msg)
}

func Infof(msg string, args ...any) {
	logf(LevelInfo, nil, msg, args...)
}

func Notice(msg string) {
	log(LevelNotice, nil, msg)
}

func Noticef(msg string, args ...any) {
	logf(LevelNotice, nil, msg, args...)
}

func Warn(msg string) {
	log(LevelWarn, nil, msg)
}

func Warnf(msg string, args ...any) {
	logf(LevelWarn, nil, msg, args...)
}

func Err(msg string) {
	log(LevelErr, nil, msg)
}

func Errf(msg string, args ...any) {
	logf(LevelErr, nil, msg, args...)
}

func Emergency(msg string) {
	log(LevelEmergency, nil, msg)
}

func Emergencyf(msg string, args ...any) {
	logf(LevelEmergency, nil, msg, args...)
}
