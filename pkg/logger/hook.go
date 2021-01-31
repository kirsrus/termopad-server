package logger

import (
	"path"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// LogrusContextHook хок к logrus
type LogrusContextHook struct{}

// Levels возвращает текущие уровни
func (hook LogrusContextHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire выбирает информацию из текущего места вызова лога, чтобы вернуть имя файла, функцию и номер
// строки вызова
func (hook LogrusContextHook) Fire(entry *logrus.Entry) error {
mainloop:
	for i := 0; ; i++ {
		if pc, file, line, ok := runtime.Caller(i); ok {
			funcName := path.Base(runtime.FuncForPC(pc).Name()) // Имя функции

			// Пропускаем вызов данной структуры
			if strings.Contains(funcName, ".LogrusContextHook.") {
				continue
			}

			// Пропускаем все служебные модули
			for _, v := range []string{"LogrusContextHook.", "logrus.", "runtime.", "testing."} {
				if strings.HasPrefix(funcName, v) {
					continue mainloop
				}
			}

			// Если дошли до сюда, значит дошли до точки логирования
			entry.Data["file"] = path.Base(file)
			entry.Data["func"] = funcName
			entry.Data["line"] = line
			break
		} else {
			break
		}
	}

	return nil
}
