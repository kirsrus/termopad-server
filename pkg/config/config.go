package config

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/jinzhu/configor"
)

var (
	config Config
	once   sync.Once
)

const FileName = "config.yaml"

// Get единажды читает и возвращает конфигурацию
func Get() *Config {
	return GetWithPath(FileName)
}

// GetWithPath единожды читает и возвращает конфигурацию
func GetWithPath(filepath string) *Config {
	once.Do(func() {
		if _, err := os.Stat(filepath); err != nil {
			log.Fatalf("файл конфигурации недоступен: %s", err)
		}
		err := configor.Load(&config, filepath)
		if err != nil {
			log.Fatalf("ошибка чтения файла конфигурации %s: %s", filepath, err)
		}
		// Корректировки значений
		config.Recognize.TimeOut = config.Recognize.TimeOut * time.Millisecond
	})
	return &config
}
