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
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

const record_length = 128

type create_handler func(io.Writer, *slog.HandlerOptions) slog.Handler

// default
var create_handler_func create_handler = func(w io.Writer, opt *slog.HandlerOptions) slog.Handler {
	return NewCustomHandler(w, opt)
}

var DailyHandlerCreateFunc create_handler = func(w io.Writer, opt *slog.HandlerOptions) slog.Handler {
	return NewCustomHandler(w, opt)
}

var StdTextHandlerCreateFunc create_handler = func(_ io.Writer, opt *slog.HandlerOptions) slog.Handler {
	return slog.NewTextHandler(os.Stdout, opt)
}

var StdJsonHandlerCreateFunc create_handler = func(_ io.Writer, opt *slog.HandlerOptions) slog.Handler {
	return slog.NewJSONHandler(os.Stdout, opt)
}

var (
	default_folder string     = "log"
	default_level  slog.Level = LevelInfo
	exe            string
	defaultLogger  atomic.Value
	lock           sync.Mutex
)

func init_folder(folder string) error {
	fi, err := os.Stat(folder)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(folder, os.ModePerm); err != nil {
				return fmt.Errorf("create folder:%v,err:%v", folder, err)
			}
		}
	} else {
		if !fi.IsDir() {
			return fmt.Errorf("%v is not dir", folder)
		}
	}
	return nil
}

var default_cron_id cron.EntryID
var c = cron.New(cron.WithSeconds())

func init_daily() {
	if err := init_folder(default_folder); err != nil {
		panic(err)
	}
	exe = path.Base(os.Args[0])
	c.Remove(default_cron_id)
	var err error
	if default_cron_id, err = c.AddFunc("0 0 0 * * *", func() {
		now := time.Now().Add(1 * time.Minute)
		filename := fmt.Sprintf("%s-%v.log", exe, now.Format(time.DateOnly))
		filename = filepath.Join(default_folder, filename)
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
		initLogger(f)
	}); err != nil {
		fmt.Println("err:", err)
	}
	c.Start()
	filename := fmt.Sprintf("%s-%v.log", exe, time.Now().Format(time.DateOnly))
	filename = filepath.Join(default_folder, filename)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	initLogger(f)
}

func Default() *logger {
	if v := defaultLogger.Load(); v != nil {
		return v.(*logger)
	} else {
		return initLogger(os.Stdout)
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
		fmt.Println("===flush:", err)
	}
}

func (this *logger) Close() {
	if this != nil {
		go this.close()
	}
}

func initLogger(w io.WriteCloser) *logger {
	lock.Lock()
	defer lock.Unlock()
	var oldLogger *logger
	if v := defaultLogger.Load(); v != nil {
		oldLogger = v.(*logger)
	}
	oldLogger.Close()
	tmpLogger := newLogger(w)
	if oldLogger == nil {
		if b := defaultLogger.CompareAndSwap(nil, tmpLogger); b {
			slog.SetDefault(tmpLogger.logger)
		}
	} else {
		if b := defaultLogger.CompareAndSwap(oldLogger, tmpLogger); b {
			slog.SetDefault(tmpLogger.logger)
		}
	}
	return tmpLogger
}

func newLogger(w io.WriteCloser) *logger {
	h := create_handler_func(w, opt)
	l := &logger{
		wc:         w,
		wg:         &sync.WaitGroup{},
		logger:     slog.New(h),
		chanRecord: make(chan *slog.Record, record_length),
	}
	l.set_level(default_level)
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
		if err := this.logger.Handler().Handle(context.Background(), *v); err != nil {
			fmt.Printf("handle msg:%+v,err:%v\n", v, err)
		}
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
	// cache = map[string]string{}
	opt = &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Any().(time.Time).Format("01/02 15:04:05"))
			}
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				switch {
				case level == LevelTrace:
					a.Value = slog.StringValue("TRACE")
				case level == LevelDebug:
					a.Value = slog.StringValue("DEBUG")
				case level == LevelInfo:
					a.Value = slog.StringValue("INFO ")
				case level == LevelNotice:
					a.Value = slog.StringValue("NOTIC")
				case level == LevelWarn:
					a.Value = slog.StringValue("WARN ")
				case level == LevelErr:
					a.Value = slog.StringValue("Err  ")
				case level == LevelEmergency:
					a.Value = slog.StringValue("EMERE")
				case level == LevelFatal:
					a.Value = slog.StringValue("FATAL")
				default:
					a.Value = slog.StringValue("no")
				}
			}
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				// if v, ok := cache[source.File]; ok {
				// 	source.File = v
				// } else {
				// 1
				// realPath := strings.TrimPrefix(source.File, pwd)
				// 2
				// files := strings.Split(source.File, string(filepath.Separator))
				// length := len(files)
				// realPath := source.File
				// if length > 3 {
				// 	files = files[length-3:]
				// 	realPath = filepath.Join(files...)
				// }
				// cache[source.File] = realPath
				// source.File = realPath

				// // 3
				// source.File = trimsamestr(source.File, pwd)
				// cache[source.File] = source.File

				//4

				source.File = filepath.Base(source.File)
				// cache[source.File] = source.File

				// }
			}
			return a
		},
	}
)

// func trimsamestr(s string, trims []rune) string {
// 	index := -1
// 	for i, v := range s {
// 		if i >= len(trims) {
// 			break
// 		}
// 		if v != trims[i] {
// 			break
// 		}
// 		index = i
// 	}
// 	if index != -1 {
// 		return s[index+1:]
// 	} else {
// 		return s
// 	}
// }

func get_skip() int {
	return default_skip + delta_skip
}

func logf(skip int, level slog.Level, f field, msg string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(skip, pcs[:])
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

func log(skip int, level slog.Level, f field, msg string) {
	var pcs [1]uintptr
	runtime.Callers(skip, pcs[:])
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

func log_any(l slog.Level, f field, msg ...any) {
	// fmt.Print("ni", 1, 3.1415926, errors.New("dd"), &Person{Name: "l", Age: 18})
	// ni 和 1粘在一起了
	// log(4, l, f, fmt.Sprint(msg...)) //这个函数格式不太对
	s := fmt.Sprintln(msg...)
	// if strings.HasSuffix(s, "\n") {
	// 	s = s[0 : len(s)-1]
	// }
	s = strings.TrimSuffix(s, "\n")
	log(get_skip()+1, l, f, s)
}

// api
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

var is_set_daily = false

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
