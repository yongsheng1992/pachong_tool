package main

import (
	"github.com/sirupsen/logrus"
	"github.com/yongsheng1992/webspiders/spiders/chinadaily"
	"github.com/yongsheng1992/webspiders/spiders/huanqiu"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

var log = logrus.New()

func main() {
	log.SetFormatter(&logrus.JSONFormatter{})
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./logs"
		_, err := os.Stat(logPath)
		if os.IsExist(err) {
			if err := os.MkdirAll(logPath, os.ModePerm); err != nil {
				panic(err)
			}
		}
	}
	logger := &lumberjack.Logger{
		Filename:   path.Join(logPath, "spider.log"),
		MaxSize:    500,   // 日志文件大小，单位是 MB
		MaxBackups: 3,     // 最大过期日志保留个数
		MaxAge:     28,    // 保留过期文件最大时间，单位 天
		Compress:   false, // 是否压缩日志，默认是不压缩。这里设置为true，压缩日志
	}

	log.SetOutput(logger)
	log.Info("start")

	ticker := time.Tick(time.Hour)

	for {
		if err := huanqiu.Run(log); err != nil {
			log.Error(err)
		}
		if err := chinadaily.Run(log); err != nil {
			log.Error(err)
		}
		select {
		case <-ticker:
			log.Info("one hour, crawling...")
		case sig := <-sigCh:
			log.Info("receive signal ", sig)
			break
		}
	}
}
