package validator

import (
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/leebenson/conform"
)

var (
	valid Validator
	once  sync.Once
)

// Validator валидатор. Инициализируется через NewValidator
type Validator struct {
	validator *validator.Validate
}

// NewValidator конструктор валидатора Validator
func NewValidator() *Validator {
	v := Validator{
		validator: validator.New(),
	}

	// Регистрируем внешние валидаторы
	if err := v.validator.RegisterValidation("websocket", validatorWebsocket); err != nil {
		panic(err)
	}

	return &v
}

// Validate валидация структуры
func (m *Validator) Validate(i interface{}) error {
	if err := conform.Strings(i); err != nil {
		return err
	}
	return m.validator.Struct(i)
}

// ValidateWithConform корректировка данных и валидация структуры
func (m *Validator) ValidateWithConform(i interface{}) error {
	if err := conform.Strings(i); err != nil {
		return err
	}
	return m.validator.Struct(i)
}

// Get единожды инициализирует и возвращает валидатор
func Get() *Validator {
	once.Do(func() {
		valid = *NewValidator()
	})
	return &valid
}
