package main

import (
	"bytes"
	"context"
	"encoding"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/ndsky1003/buffer/v2"
)

var pool = buffer.NewBufferPool(buffer.Options().SetCalibratedSz(0).SetMinSize(512))

// FastTextHandler 高性能文本 Handler
type FastTextHandler struct {
	w            io.Writer
	opts         slog.HandlerOptions
	mu           sync.Mutex
	preformatted []byte // 预序列化的属性 (WithAttrs)
	groupPrefix  string // 组前缀 (WithGroup)
}

// NewFastTextHandler 构造函数
func NewFastTextHandler(w io.Writer, opts *slog.HandlerOptions) *FastTextHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			AddSource: true,
		}
	}
	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}

	return &FastTextHandler{
		w:    w,
		opts: *opts,
	}
}

func (h *FastTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// Handle 核心热点路径
func (h *FastTextHandler) Handle(_ context.Context, r slog.Record) error {
	// 1. 获取原生 buffer (0 allocation)
	buf := pool.Get()
	// 2. 归还 (Defer 在极高性能场景有微小开销，但在 IO 操作前可忽略)
	defer pool.Put(buf)

	// 3. 拼接日志
	// time=...
	if !r.Time.IsZero() {
		// buf.WriteString("time=")
		writeTime(buf, r.Time)
		buf.WriteByte(' ')
	}

	// level=...
	// buf.WriteString("level=")
	buf.WriteString(r.Level.String())
	buf.WriteByte(' ')

	// source=... (只有开启才计算)
	if h.opts.AddSource && r.PC != 0 {
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

// WithAttrs 优化：预先将 Attr 序列化为 []byte
func (h *FastTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := *h // 浅拷贝

	// 使用池子里的 buffer 来做临时拼接
	preBuf := pool.Get()
	defer pool.Put(preBuf)

	// 先把旧的拷进去
	if len(h.preformatted) > 0 {
		preBuf.Write(h.preformatted)
	}
	// 再追加新的
	for _, a := range attrs {
		h2.appendAttr(preBuf, a, h.groupPrefix)
	}

	// 必须 Copy 出去，因为 Handler 生命周期很长，不能引用池子里的内存
	h2.preformatted = make([]byte, preBuf.Len())
	copy(h2.preformatted, preBuf.Bytes())

	return &h2
}

// WithGroup 处理组前缀
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
	writeValue(b, a.Value)
}

func writeValue(b *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		writeString(b, v.String())
	case slog.KindInt64:
		// strconv.AppendInt 是零分配的，但 bytes.Buffer 没有直接接收 []byte 的 AppendInt
		// 为了性能，我们转成 string 写入，Go 的编译器对这种短字符串转换有优化
		// 极致优化可以用 b.Write(strconv.AppendInt(tempBuf, ...)) 但需要管理 tempBuf
		b.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		b.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindFloat64:
		b.WriteString(strconv.FormatFloat(v.Float64(), 'f', -1, 64))
	case slog.KindBool:
		if v.Bool() {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case slog.KindDuration:
		b.WriteByte('"')
		b.WriteString(v.Duration().String())
		b.WriteByte('"')
	case slog.KindTime:
		b.WriteByte('"')
		writeTime(b, v.Time())
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

// func writeTime(b *bytes.Buffer, t time.Time) {
// 	// RFC3339Nano 格式
// 	b.WriteString(t.Format(time.DateTime))
// }

func writeTime(b *bytes.Buffer, t time.Time) {
	// 🚀 快：使用缓存 + 零分配拼接
	// bytes.Buffer 内部暴露不了 []byte 用于 append，
	// 但我们可以用 b.Write(AppendTime(tempBuf, t))
	// 为了极致性能，建议你的 DynamicBufferPool 里的 buffer 直接就是 []byte
	// 或者用以下技巧：

	// 因为 AppendTime 需要 append 到一个 slice 上
	// 我们利用 buffer 的 Write 接口
	// 但 AppendTime 返回的是新 slice，这可能导致 allocation

	// 【终极优化】：
	// 修改 DynamicBufferPool，让 Get() 返回 *[]byte 或者 自定义结构体
	// 但既然我们现在用的是 *bytes.Buffer，我们可以偷懒利用一个小 trick：

	// 方案 A：直接用 bytes.Buffer 的 WriteString
	// 不行，我们想要 append。

	// 方案 B：AppendTime 改写为接受 *bytes.Buffer
	fastAppendTime(b, t)
}

func fastAppendTime(b *bytes.Buffer, t time.Time) {
	unixSec := t.Unix()
	cache := globalTimeCache.Load()

	if cache != nil && cache.unixSec == unixSec {
		// 命中缓存：直接写入预先格式化好的字节
		b.Write(cache.formatted)
	} else {
		updateTimeCache(t)
		// 递归重试
		fastAppendTime(b, t)
		return
	}

	// 处理毫秒
	b.WriteByte('.')
	// 手动优化：小于 100000 的补零逻辑太繁琐，
	// 这里可以直接用 strconv.AppendInt 到临时 buffer，或者直接 copy 算法
	// 简单高效写法：
	// 使用 strconv.AppendInt 并不分配内存
	// 但 bytes.Buffer 没有 AppendInt。
	// 我们只能：
	nano := t.Nanosecond() / 1000 // 微秒

	// 这是一个极简的手动 int to string (6位固定)
	// 避免了 strconv 的通用开销
	tmp := [6]byte{}
	val := nano
	for i := 5; i >= 0; i-- {
		tmp[i] = byte(val%10 + '0')
		val /= 10
	}
	b.Write(tmp[:])

	b.WriteByte('Z')
}

func writeSource(b *bytes.Buffer, pc uintptr) {
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.File == "" {
		return
	}
	_, file := filepath.Split(f.File)
	// b.WriteByte('"')
	b.WriteString(file)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(f.Line))
	// b.WriteByte('"')
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
