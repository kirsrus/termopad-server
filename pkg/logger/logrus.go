package logger

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"sync"
	"time"
)

const (
	RotateMaxSize    = 30 // MB
	RotateLocalTime  = true
	RotateMaxAge     = 365 // Дней
	RotateMaxBackups = 10 // Колличество файлов
	RotateCompress   = true
)

var (
	logger *logrus.Logger
	once   sync.Once
)

// Config конфигурация лога
type Config struct {
	File    string
	Level   logrus.Level
	Console bool
}

// Get быстрый конфиг на консоль
func Get(level logrus.Level) *logrus.Logger {
	return GetWithConfig(Config{
		File:    "",
		Level:   level,
		Console: true,
	})
}

// GetWithConfig лоигрование с конфигурацией
func GetWithConfig(config Config) *logrus.Logger {
	once.Do(func() {
		//stdOut := os.Stdout
		log := logrus.New()
		log.Level = config.Level
		log.Formatter = &logrus.TextFormatter{
			DisableColors:   false,
			TimestampFormat: "2006.01.02 15:04:05",
			//ForceColors:     true,
		}
		log.Out = io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   config.File,
			MaxSize:    RotateMaxSize, // MB
			MaxAge:     RotateMaxAge,  // Day
			MaxBackups: RotateMaxBackups,
			LocalTime:  RotateLocalTime,
			Compress:   RotateCompress,
		})

		//if config.Console || config.File == "" {
		//	log.Out = stdOut
		//} else {
		//	file, err := os.Create(config.File)
		//	if err != nil {
		//		log.Out = stdOut
		//	} else {
		//		log.Out = file
		//	}
		//}
		log.AddHook(LogrusContextHook{})
		// Вступительная запись на высоком уровне
		//prevLogLevel := log.Level
		//log.Level = logrus.InfoLevel
		//log.Printf("----------===== начало записи в лог %s =====----------", time.Now())
		//log.Level = prevLogLevel
		log.Println("----------===== начало записи в лог %s =====----------", time.Now())
		logger = log
	})
	return logger
}

//func GetWithPath(filename string) *logrus.Logger {
//	once.Do(func() {
//		var err error
//		stdOut := os.Stdout
//		cfg := config.Get()
//		log := logrus.New()
//		log.Level, err = logrus.ParseLevel(cfg.Log.Level)
//		if err != nil {
//			log.Level = logrus.WarnLevel
//		}
//		log.Formatter = &logrus.TextFormatter{
//			DisableColors:   false,
//			TimestampFormat: "2006.01.02 15:04:05",
//		}
//		if cfg.Log.Console || filename == "" {
//			log.Out = stdOut
//		} else {
//			file, err := os.Create(filename)
//			if err != nil {
//				log.Out = stdOut
//			} else {
//				log.Out = file
//			}
//		}
//		log.AddHook(LogrusContextHook{})
//		// Вступительная запись на высоком уровне
//		prevLogLevel := log.Level
//		log.Level = logrus.InfoLevel
//		log.Printf("----------===== начало записи в лог %s =====----------", time.Now())
//		log.Level = prevLogLevel
//		logger = log
//	})
//	return logger
//}
