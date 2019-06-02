package record

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func initLog() error {
	baseLogPath := path.Join("./log", "record.log")
	writer, err := rotatelogs.New(
		baseLogPath+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(baseLogPath),      // 生成软链，指向最新日志文件
		rotatelogs.WithMaxAge(7*24*time.Hour),     // 文件最大保存时间
		rotatelogs.WithRotationTime(24*time.Hour), // 日志切割时间间隔
	)
	if err != nil {
		return err
	}

	switch level := config.Log.Level; level {
	/*
	   如果日志级别不是debug就不要打印日志到控制台了
	*/
	case "debug":
		log.SetLevel(logrus.DebugLevel)
		log.SetOutput(os.Stdout)
	case "info":
		setNull()
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		setNull()
		log.SetLevel(logrus.WarnLevel)
	case "error":
		setNull()
		log.SetLevel(logrus.ErrorLevel)
	default:
		setNull()
		log.SetLevel(logrus.InfoLevel)
	}

	lfHook := lfshook.NewHook(lfshook.WriterMap{
		logrus.DebugLevel: writer,
		logrus.InfoLevel:  writer,
		logrus.WarnLevel:  writer,
		logrus.ErrorLevel: writer,
		logrus.FatalLevel: writer,
		logrus.PanicLevel: writer,
	}, &logrus.TextFormatter{})
	log.AddHook(lfHook)

	return nil
}

func setNull() {
	src, err := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		fmt.Println("err", err)
	}
	writer := bufio.NewWriter(src)
	log.SetOutput(writer)
}
