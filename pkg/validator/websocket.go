package validator

import (
	"net/url"

	"github.com/go-playground/validator/v10"
)

// Валидатор корректной ссылки на WebSocket
func validatorWebsocket(fl validator.FieldLevel) bool {
	address, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	addr, err := url.Parse(address)
	if err != nil {
		return false
	}
	if addr.Scheme != "ws" {
		return false
	}
	return true
}
