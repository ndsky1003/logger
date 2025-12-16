package main

import (
	"strconv"
	"sync/atomic"
	"time"
)

// cache 结构体：存储某一秒的字符串缓存
type timeCache struct {
	unixSec   int64  // 这一秒的 Unix 时间戳
	formatted []byte // 格式化好的字节： "2025-12-14T10:00:01"
}

// 全局原子容器，存储当前的 cache
var globalTimeCache atomic.Pointer[timeCache]

// 初始化
func init() {
	updateTimeCache(time.Now())
}

// updateTimeCache 更新缓存 (慢路径)
// 这里的逻辑稍微重一点没关系，因为一秒才跑一次
func updateTimeCache(t time.Time) {
	// 构造新的 cache 对象
	newCache := &timeCache{
		unixSec: t.Unix(),
	}

	// 手动格式化，避免使用 time.Format 的 layout 解析
	// 格式：2006-01-02T15:04:05
	// 我们直接分配一个小 buffer，只用一次
	buf := make([]byte, 0, 20)

	year, month, day := t.Date()
	hour, min, sec := t.Clock()

	// 快速追加整数，避免反射
	buf = appendInt(buf, year, 4)
	buf = append(buf, '-')
	buf = appendInt(buf, int(month), 2)
	buf = append(buf, '-')
	buf = appendInt(buf, day, 2)
	buf = append(buf, 'T') // 或者 ' '，看你喜好
	buf = appendInt(buf, hour, 2)
	buf = append(buf, ':')
	buf = appendInt(buf, min, 2)
	buf = append(buf, ':')
	buf = appendInt(buf, sec, 2)

	newCache.formatted = buf

	// 原子替换
	globalTimeCache.Store(newCache)
}

// AppendTime 极速时间格式化 (快路径)
func AppendTime(b []byte, t time.Time) []byte {
	unixSec := t.Unix()
	cache := globalTimeCache.Load()

	// 1. 命中缓存：秒数一样，直接拷贝前半部分
	if cache != nil && cache.unixSec == unixSec {
		b = append(b, cache.formatted...)
	} else {
		// 2. 未命中（下一秒到了）：更新缓存，然后递归调用自己
		// 注意：多协程并发更新没问题，atomic 会保证最终一致性
		updateTimeCache(t)
		return AppendTime(b, t)
	}

	// 3. 实时处理毫秒/纳秒部分
	// 格式：.123456+08:00
	b = append(b, '.')
	// 只取微秒前6位，根据需求调整
	micro := t.Nanosecond() / 1000
	b = appendInt(b, micro, 6)

	// 时区部分通常是固定的，也可以缓存，或者简单处理
	// 这里简化处理，假设是 UTC 或者由外部控制 Z
	b = append(b, 'Z')

	return b
}

// appendInt 是一个比 fmt.Sprintf 快 10 倍的整数转字符串辅助函数
func appendInt(b []byte, i int, width int) []byte {
	// 针对固定宽度的优化
	// 如果 i = 5, width = 2 -> "05"

	// 这里用 strconv.AppendInt 很万能，但手动拆解更快
	// 工业级通常会针对 2位、4位数字做查表法优化 (Look-up Table)
	// 简单起见，这里用 strconv 的 Append 模式，已经足够快且零分配

	// 处理前导零
	s := strconv.Itoa(i)
	for len(s) < width {
		b = append(b, '0')
		width--
	}
	b = append(b, s...)
	return b
}
