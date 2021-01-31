package db

import (
	"time"

	"github.com/kirsrus/termopad/server2/model"
)

type (
	// GormModelUnscoped модель эквивалент gorm.Model без сохранения удалений
	GormModelUnscoped struct {
		ID        int `gorm:"primaryKey"`
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	// Config конфигурация программы
	Config struct {
		GormModelUnscoped
		MaxTemperature int
		MinTemperature int
	}
)

// TableName имя таблицы
func (Config) TableName() string {
	return "config"
}

type (
	// Person описывает персону
	Person struct {
		GormModelUnscoped
		Wigand       int
		Family       string
		Name         string
		MiddleName   string
		Organization string
		Department   string
		Position     string
	}
)

// TableName имя таблицы
func (Person) TableName() string {
	return "persons"
}

// Update обновление данных текущей персоны переданной в person
func (m *Person) Update(person Person) {
	m.Family = person.Family
	m.Name = person.Name
	m.MiddleName = person.MiddleName
	m.Organization = person.Organization
	m.Department = person.Department
	m.Position = person.Position
}

// ToPerson маппинг данных в структуру Person
func (m Person) ToPerson() model.Person {
	person := model.Person{
		CreateAt:     &m.CreatedAt,
		UpdateAt:     &m.UpdatedAt,
		Wigand:       model.Wigand{ID: uint(m.Wigand)},
		Family:       m.Family,
		Name:         m.Name,
		MiddleName:   m.MiddleName,
		Organization: m.Organization,
		Department:   m.Department,
		Position:     m.Position,
	}
	return person
}

// FromPerson заполняет текущую структуру из структуры model.Person
func (m *Person) FromPerson(person model.Person) {
	*m = Person{
		Wigand:       int(person.Wigand.ID),
		Family:       person.Family,
		Name:         person.Name,
		MiddleName:   person.MiddleName,
		Organization: person.Organization,
		Department:   person.Department,
		Position:     person.Position,
	}
}

type (
	// Termopad информация о термопадах
	Termopad struct {
		GormModelUnscoped
		// В качестве CabinaID используется уникальный ID из базы данных СУДОС (может быть 0)
		// и предоставляется СУДОС
		CabinaID uint
		Name     string
		// URL подключения к WebSocket термопада и имеет полный формат "ws://192.168.36.3:8000/feed"
		URL               string
		Descripton        string
		RerconnectTimeout int
	}
)

// TableName имя таблицы
func (Termopad) TableName() string {
	return "termopads"
}

type (
	// Temperature логирование температуры для Person
	Temperature struct {
		GormModelUnscoped
		// В качестве PersonID используется полный номер Wigand
		PersonID int
		// В качестве TermopadID используется ID кабины, присваиваемый БД
		TermopadID  int
		Temperature float64
		ImageName   string
	}
)

// TableName имя таблицы
func (Temperature) TableName() string {
	return "temperature_log"
}
