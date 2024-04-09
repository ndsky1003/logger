package logger

import "testing"

// func TestLogger(t *testing.T) {
// 	defer Flush()
// 	// SetFolder("logs")
// 	// SetLevel(LevelWarn)
// 	Debug("debug")
// 	// Info("info1")
// 	// Warn("warn")
// 	// Err("err")
// 	// Fields().Add("key1", "value1").Info("infomsg")
// }

func TestFatal(t *testing.T) {
	defer Flush()
	// SetLevel(LevelFatal)
	Fatal("fatal")
}

// BenchmarkLogger-12    	 2540012	      4563 ns/op
// 速度全部限制在了文件io上
func BenchmarkLogger(b *testing.B) {
	// defer Flush()
	for i := 0; i < b.N; i++ {
		Info("info", i)
	}
}
