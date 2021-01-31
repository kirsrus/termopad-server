package model

import (
	"github.com/juju/errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TermopadInfo описывает технические данные термопада
type TermopadInfo struct {
	ID           uint   `validate:"required"`
	URL          string `conform:"trim" validate:"required,websocket"`
	SudosID      uint
	Name         string `conform:"trim" validate:"required"`
	SerialNumber uint
	Description  string `conform:"trim"`
}

// TermopadAction событие в WebSocket канале термопада
type TermopadAction struct {
	// Тип события:
	//    newImage - доступно новое иображение
	Action string `json:"action"`
	// Дата собятия в формате "2020-11-27T12:37:54.838079"
	Timestamp string `json:"timestamp"`
	// Имя сохранённого на термопаде файла формата "27-11-2020--12-37-54--Unknown--36.5.jpg"
	FileName string `json:"filename"`
	// Дата сохранения в формате "11/27/2020 12:37:54"
	Date string `json:"date"`
	// Номер карты, или ошибка "Unknown"
	CardNumber string `json:"card_number"`
	// Температура в формате "36.5"
	Temperature string `json:"temperature"`
}

// Validate валидация
func (m TermopadAction) Validate() error {
	if m.Action == "" {
		return errors.New("не задан параметр Action")
	}
	if m.Timestamp == "" {
		return errors.New("не задан параметр Timestamp")
	}
	if m.FileName == "" {
		return errors.New("не задан параметр FileName")
	}
	return nil
}

// TermopadFileName распарсенное имя файла на термопаде
type TermopadFileName struct {
	Time        time.Time
	Wigand      Wigand
	Temperature float64
	FileName    string
}

// Parse заполняет структуру из имени файла
func (m *TermopadFileName) Parse(fileName string) error {
	*m = TermopadFileName{}
	var err error

	re := regexp.MustCompile(`(\d+-\d+-\d+--\d+-\d+-\d+)--([\w\d]+)--(-?)(\d+\.\d+)\.jpg`)
	match := re.FindStringSubmatch(fileName)
	if len(match) == 0 {
		return errors.Errorf("формат имени файла \"%s\" не распознан", fileName)
	}

	m.Time, err = time.Parse("02-01-2006--15-04-05", match[1])
	if err != nil {
		return errors.Errorf("некорректный формат записи времени \"%s\" в имени файла: %s", match[1], fileName)
	}

	if match[3] != "" { // Минусовое значение температры
		m.Temperature = 0.0
	} else {
		temp, err := strconv.ParseFloat(match[4], 32)
		if err != nil {
			return errors.Errorf("в имени файла \"%s\" не удалось расознать температуту \"%s\"", fileName, match[3])
		}
		m.Temperature = temp
	}

	number, err := strconv.Atoi(match[2])
	if err != nil && strings.ToLower(match[2]) != "unknown" {
		return errors.Errorf("в имени файла \"%s\" не удалось распознать номер виганда \"%s\"", fileName, match[2])
	}
	m.Wigand = Wigand{ID: uint(number)}
	m.FileName = fileName

	return nil
}

// TermopadTemperatureEvent событие о температуре с термопада
type TermopadTemperatureEvent struct {
	CreateAt    *time.Time
	// Абслоютный путь до сохранённого изображения
	Image       string
	Info        TermopadInfo
	Temperature TemperatureEvent
}
