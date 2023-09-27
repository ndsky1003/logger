# logger
base on slog
该库并非同步打印，不会阻塞进程

#### usage
```golang
func main(){
        defer logger.Close() //chan 当的锁，为了保证其全部写入
    }




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

#### 切换handler
>默认的是自定义的CustomHandler,handler在使用handle这个函数，只需要实现该函数即可
```golang
	logger.SetCreateHandler(func(w io.Writer, opt *slog.HandlerOptions) slog.Handler {
		// return slog.NewJSONHandler(w, opt)
		return slog.NewTextHandler(w, opt)
	})

