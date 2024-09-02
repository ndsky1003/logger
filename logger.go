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
	"sync"
	"sync/atomic"
	"time"

	"github.com/ndsky1003/buffer"
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

func get_skip() int {
	return default_skip + delta_skip
}

func logf(skip int, level slog.Level, f Field, msg string, args ...any) {
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

func log(skip int, level slog.Level, f Field, msgs ...any) {
	s := ""
	if wrap_func_obj != nil {
		s = wrap_func_obj(msgs...)
	} else {
		buf := buffer.Get()
		defer buf.Release()
		for _, v := range msgs {
			str := fmt.Sprintf("%+v ", v)
			buf.WriteString(str)
		}
		s = buf.String()
	}
	var pcs [1]uintptr
	runtime.Callers(skip, pcs[:])
	r := slog.NewRecord(time.Now(), level, s, pcs[0])
	if f != nil {
		attrs := make([]slog.Attr, 0, len(f))
		for k, v := range f {
			attrs = append(attrs, slog.Attr{Key: k, Value: v})
		}
		r.AddAttrs(attrs...)
	}
	Default().log(&r)
}

func log_any(l slog.Level, f Field, msgs ...any) {
	log(get_skip()+1, l, f, msgs...)
}

type warp_func func(...any) string

var wrap_func_obj warp_func

var is_set_daily = false
