package controller

import (
	"github.com/kirsrus/termopad/server2/model"
)

// TermopadCtl контроллер управления термопадами
//go:generate mockery --dir . --name TermopadCtl --output ./mocks
type TermopadCtl interface {
	// Ожидает очередное сообщение от текромпада и возвращает в своём результате полученные данные.
	EmmitTemperature() (*model.TermopadTemperatureEvent, error)
}
