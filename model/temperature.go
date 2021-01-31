package model

import "time"

// Temperature информация о температуре
type Temperature struct {
	TermopadID  uint `validate:"required"`
	Wigand      Wigand
	Temperature float64 `validate:"required"`
	ImagePath   string  `conform:"trim"`
	Image       []byte
}

// TemperatureInfo описывает событие о температуре
type TemperatureEvent struct {
	Temperature float64 `validate:"required"`
	Wigand      Wigand
	Image       []byte
}

// TemperatureMetric элемент метрики температуры (для отображения в графиках)
type TemperatureMetric struct {
	Date           time.Time
	Temperature    float64
	TemperatureMax float64
	TemperatureMin float64
	Image          string
	Person         Person
	Termopad       TermopadInfo
}
