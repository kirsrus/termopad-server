package model

import (
	"fmt"
	"strconv"
)

// Wigand описание карты виганд. Создаётся через конструктор NewWigand
type Wigand struct {
	// Общий номер карты виганд (собранный из фасалити и номера)
	ID uint `validate:"required"`
}

// NewWigand конструктор структуры Wigand.
func NewWigand(ID int) Wigand {
	return Wigand{ID: uint(ID)}
}

// Парсинго номера вигадна на фасалити и номер
func (m Wigand) parse() (uint, uint) {
	if m.ID == 0 {
		return 0, 0
	}
	fasality := m.ID >> 16
	num := m.ID>>16<<16 ^ m.ID
	return fasality, num
}

// Fasality фасалити виганда
func (m Wigand) Fasality() uint {
	fasality, _ := m.parse()
	return fasality
}

// Number номера карты виганд
func (m Wigand) Number() uint {
	_, number := m.parse()
	return number
}

// Parse парсит в общий номер (ID) из фасалити и номера
func (m *Wigand) Parse(fasality uint, number uint) {
	if fasality == 0 && number == 0 {
		m.ID = 0
		return
	}
	s, _ := strconv.ParseUint(fmt.Sprintf("%b%016b", fasality, number), 2, 32)
	m.ID = uint(s)
}

// IsEmpty данные виганда не заполнены
func (m Wigand) IsEmpty() bool {
	return m.ID == 0
}

// String краткое описание
func (m Wigand) String() string {
	if m.ID == 0 {
		return "0-0 (0)"
	}
	return fmt.Sprintf("%d-%d (%d)", m.Fasality(), m.Number(), m.ID)
}
