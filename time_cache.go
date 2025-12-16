package logger

import (
	"sync/atomic"
	"time"
)

// cache 结构体：存储某一秒的字符串缓存
type timeCache struct {
	unixSec   int64  // 这一秒的 Unix 时间戳
	formatted []byte // 格式化好的字节： "2025-12-14T10:00:01"
}

func updateTimeCache(cache *atomic.Pointer[timeCache], t time.Time) []byte {
	newCache := &timeCache{
		unixSec: t.Unix(),
	}

	buf := make([]byte, 0, 19)
	year, month, day := t.Date()
	hour, min, sec := t.Clock()

	// 快速追加整数，避免反射
	buf = appendInt(buf, year, 4)
	buf = append(buf, '-')
	buf = appendInt(buf, int(month), 2)
	buf = append(buf, '-')
	buf = appendInt(buf, day, 2)
	buf = append(buf, ' ') // 或者 ' '，看你喜好
	buf = appendInt(buf, hour, 2)
	buf = append(buf, ':')
	buf = appendInt(buf, min, 2)
	buf = append(buf, ':')
	buf = appendInt(buf, sec, 2)
	newCache.formatted = buf

	// 原子替换
	cache.Store(newCache)
	return buf
}

func appendInt(b []byte, i int, width int) []byte {
	u := uint(i)

	if width == 2 {
		// 优化：针对月、日、时、分、秒 (00-99)
		// 直接计算十位和个位，一次性 append
		d1 := u / 10
		d0 := u % 10
		return append(b, byte(d1)+'0', byte(d0)+'0')
	}

	if width == 4 {
		// 优化：针对年份 (0000-9999)
		// 这里的除法会被编译器优化为乘法逆元，非常快
		q1 := u / 100
		d0 := u % 100 // 后两位
		d1 := q1      // 前两位

		return append(b,
			byte(d1/10)+'0', byte(d1%10)+'0',
			byte(d0/10)+'0', byte(d0%10)+'0',
		)
	}

	// 理论上不会走到这里，保留一个通用兜底或者 panic
	return b
}
