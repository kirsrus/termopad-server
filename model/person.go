package model

import (
	"time"
)

// Person описывает персону
type Person struct {
	CreateAt     *time.Time
	UpdateAt     *time.Time
	Wigand       Wigand
	Family       string `conform:"trim" validate:"required"`
	Name         string `conform:"trim" validate:"required"`
	MiddleName   string `conform:"trim"`
	Organization string `conform:"trim"`
	Department   string `conform:"trim"`
	Position     string `conform:"trim"`
	// Изображение из базы данных СУДОС
	Image []byte
}
