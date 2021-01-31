package model

import (
	"time"
)

// TemperatureChange событие замера термпературы у новой персоны
type TemperatureChange struct {
	// ID терминала
	ID           uint
	CreateAt     time.Time
	Temperature  float64
	Image        string
	Wigand       Wigand
	NameFirst    string
	NameMiddle   string
	NameLast     string
	Organization string
	Departament  string
	Postion      string
}

// TemperatureUpdate событие обновления данных о зафиксированной TemperatureChange температуры
type TemperatureUpdate struct {
	// ID терминала
	ID           uint
	UpdateAt     time.Time
	Wigand       Wigand
	NameFirst    string
	NameMiddle   string
	NameLast     string
	Organization string
	Departament  string
	Postion      string
}
