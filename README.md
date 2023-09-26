# logger
base on slog
该库并非同步打印，不会阻塞进程

```golang

func main() {
        logger.SetLevel(slog.LevelInfo) // 设置打印等级
	logger.Info("nihao")
	logger.Infof("nihao:%s", "ppxia")
	test()
	logger.Fields().Add("age", slog.Int64Value(18)).Info("show age")
	select {}
}

func test() {
	ff := logger.Fields()
	defer ff.Info("ppxia")
	ff.Add("nihao", slog.StringValue("123"))
	ff.Add("nihao1", slog.StringValue("123"))
	ff.Add("nihao2", slog.StringValue("123"))
	ff.Add("nihao3", slog.StringValue("123"))
}
```
