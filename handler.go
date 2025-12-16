package logger

import (
	"bytes"
	"context"
	"encoding"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ndsky1003/buffer/v2"
)

// 1. 接口守卫
var _ slog.Handler = (*FastTextHandler)(nil)

var pool = buffer.NewBufferPool(buffer.Options().SetCalibratedSz(0).SetMinSize(512))

// FastTextHandler 高性能文本 Handler
type FastTextHandler struct {
	w            io.Writer
	opt          *Option
	level        *slog.LevelVar
	addSource    *atomic.Bool
	mu           *sync.Mutex
	timeCache    *atomic.Pointer[timeCache]
	preformatted []byte // 预序列化的属性 (WithAttrs)
	groupPrefix  string // 组前缀 (WithGroup)
}

// NewFastTextHandler 构造函数
func NewFastTextHandler(opts ...*Option) *FastTextHandler {
	opt := Options().
		SetWriter(os.Stdout).
		SetAddSource(false).
		SetLevel(LevelInfo).
		Merge(opts...)

	c := &FastTextHandler{
		w:         opt.w,
		opt:       &opt,
		level:     &slog.LevelVar{},
		addSource: &atomic.Bool{},
		timeCache: &atomic.Pointer[timeCache]{},
		mu:        &sync.Mutex{},
	}
	updateTimeCache(c.timeCache, time.Now())
	c.level.Set(*opt.level)
	c.addSource.Store(*opt.addSource)
	return c
}

func (h *FastTextHandler) SetLevel(level Level) {
	h.level.Set(level)
}
func (h *FastTextHandler) SetAddSource(b bool) {
	h.addSource.Store(b)
}

func (h *FastTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	opt := h.opt
	if opt.forcedebugfn != nil && opt.forcedebugfn(ctx) {
		return true
	}
	return level >= h.level.Level()
}

// Handle 核心热点路径
func (h *FastTextHandler) Handle(ctx context.Context, r slog.Record) error {
	opt := h.opt
	if opt.extractorfn != nil {
		opt.extractorfn(ctx, &r)
	}
	// 1. 获取原生 buffer (0 allocation)
	buf := pool.Get()
	// 2. 归还 (Defer 在极高性能场景有微小开销，但在 IO 操作前可忽略)
	defer pool.Put(buf)

	// 3. 拼接日志
	// time=...
	if !r.Time.IsZero() {
		// buf.WriteString("time=")
		writeTime(h.timeCache, buf, r.Time)
		buf.WriteByte(' ')
	}

	// level=...
	// buf.WriteString("level=")
	buf.WriteString(r.Level.String())
	buf.WriteByte(' ')

	// source=... (只有开启才计算)
	if h.addSource.Load() && r.PC != 0 {
		// buf.WriteString("source=")
		writeSource(buf, r.PC)
		buf.WriteByte(' ')
	}

	// msg=...
	// brf.WriteString("msg=")
	writeString(buf, r.Message)

	// 4. 追加预计算的属性 (WithAttrs 产生)
	if len(h.preformatted) > 0 {
		buf.Write(h.preformatted)
	}

	// 5. 追加当前 Record 的属性
	if r.NumAttrs() > 0 {
		r.Attrs(func(a slog.Attr) bool {
			h.appendAttr(buf, a, h.groupPrefix)
			return true
		})
	}

	buf.WriteByte('\n')

	// 6. 写入 IO (加锁，防止多协程写入混乱)
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *FastTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := *h // 浅拷贝 copy_on_write 没有问题
	preBuf := pool.Get()
	defer pool.Put(preBuf)
	for _, a := range attrs {
		h2.appendAttr(preBuf, a, h.groupPrefix)
	}
	l_old := len(h.preformatted)
	h2.preformatted = make([]byte, l_old+preBuf.Len())
	if l_old > 0 {
		copy(h2.preformatted, h.preformatted)
	}
	copy(h2.preformatted, preBuf.Bytes())
	return &h2
}

func (h *FastTextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	if h2.groupPrefix != "" {
		h2.groupPrefix += name + "."
	} else {
		h2.groupPrefix = name + "."
	}
	return &h2
}

// -----------------------------------------------------------------------------
// 序列化辅助函数
// -----------------------------------------------------------------------------

func (h *FastTextHandler) appendAttr(b *bytes.Buffer, a slog.Attr, prefix string) {
	// 1. 处理 ReplaceAttr 逻辑
	if h.opt.replaceAttr != nil {
		// groups 参数目前你的实现没维护，如果是扁平结构传 nil 即可
		// 也就是 h.opt.replaceAttr(nil, a)
		// 但为了严谨，最好在 Handler 里维护 groups 栈，如果暂不支持 group，至少传 nil
		a = h.opt.replaceAttr(nil, a)
	}

	// ReplaceAttr 可能会返回空的 Attr 意为删除该字段
	if a.Equal(slog.Attr{}) {
		return
	}

	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	b.WriteByte(' ') // 属性前加空格
	if prefix != "" {
		b.WriteString(prefix)
	}
	b.WriteString(a.Key)
	b.WriteByte('=')
	h.writeValue(b, a.Value)
}

func (h *FastTextHandler) writeValue(b *bytes.Buffer, v slog.Value) {
	var buf [64]byte
	switch v.Kind() {
	case slog.KindString:
		writeString(b, v.String())
	case slog.KindInt64:
		// AppendInt 将结果写入 buf[:0]，不产生堆分配
		b.Write(strconv.AppendInt(buf[:0], v.Int64(), 10))
	case slog.KindUint64:
		b.Write(strconv.AppendUint(buf[:0], v.Uint64(), 10))
	case slog.KindFloat64:
		b.Write(strconv.AppendFloat(buf[:0], v.Float64(), 'f', -1, 64))
	case slog.KindBool:
		b.Write(strconv.AppendBool(buf[:0], v.Bool()))
	case slog.KindDuration:
		b.WriteByte('"')
		b.WriteString(v.Duration().String())
		b.WriteByte('"')
	case slog.KindTime:
		b.WriteByte('"')
		writeTime(h.timeCache, b, v.Time())
		b.WriteByte('"')
	case slog.KindAny:
		if tm, ok := v.Any().(encoding.TextMarshaler); ok {
			data, err := tm.MarshalText()
			if err == nil {
				writeString(b, string(data))
				return
			}
		}
		writeString(b, fmt.Sprint(v.Any()))
	default:
		writeString(b, v.String())
	}
}

func writeTime(tc *atomic.Pointer[timeCache], b *bytes.Buffer, t time.Time) {
	fastAppendTime(tc, b, t)
}

func fastAppendTime(tc *atomic.Pointer[timeCache], b *bytes.Buffer, t time.Time) {
	unixSec := t.Unix()
	cache := tc.Load()

	if cache != nil && cache.unixSec == unixSec {
		// 命中缓存：直接写入预先格式化好的字节
		b.Write(cache.formatted)
	} else {
		formatted := updateTimeCache(tc, t)
		b.Write(formatted)
	}

}

// 全局或绑定在 Handler 上的缓存
var pcCache sync.Map // key: uintptr, value: []byte

func writeSource(b *bytes.Buffer, pc uintptr) {
	// 1. 查缓存
	if v, ok := pcCache.Load(pc); ok {
		b.Write(v.([]byte))
		return
	}

	// 2. 缓存未命中，执行昂贵的查找
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.File == "" {
		return
	}
	_, file := filepath.Split(f.File)

	// 格式化为 "file.go:123"
	var tmp [32]byte
	result := make([]byte, 0, len(file)+10)
	result = append(result, file...)
	result = append(result, ':')
	result = append(result, strconv.AppendInt(tmp[:0], int64(f.Line), 10)...)

	// 3. 写入缓存并输出
	pcCache.Store(pc, result)
	b.Write(result)
}

// writeString 处理需要转义的字符串
func writeString(b *bytes.Buffer, s string) {
	if needsQuoting(s) {
		b.WriteString(strconv.Quote(s))
	} else {
		b.WriteString(s)
	}
}

func needsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		// logfmt 规范：空格、等号、引号需要转义
		if c == ' ' || c == '=' || c == '"' || c < ' ' || c > '~' {
			return true
		}
	}
	return false
}
