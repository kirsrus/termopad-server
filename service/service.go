package service

import (
	"github.com/kirsrus/termopad/server2/model"
)

// WebSvc серис общения с WEB интерфейсом
//go:generate mockery --dir . --name WebSvc --output ./mocks
type WebSvc interface {
	// Хэндлер показа основной страницы рабочего стола
	Static(string)
	// Хэндлер общения с API через GraphQL
	GraphQLApi(string)
	// Хэндлер общения с Playground GraphQL
	GraphQLPlayground(string)
	// Хэндлер возвращения изображения персоны. Имя файла изображения ищется в параметре :name
	TemperatureImage(string)
	// Показать изображение персоны
	PersonImage(string)
	// Отсылка события измерения температуры
	TemperatureChanged(model.TemperatureChange)
}

// SudosSvc репозиторий общения с СУДОС
//go:generate mockery --dir . --name SudosSvc --output ./mocks
type SudosSvc interface {
	// Запрашивает даныне персоны по номеру Wigand.
	Person(model.Wigand) (*model.Person, error)
	// Устанавливает температуру персоны.
	SetPersonTemperature(model.Person, model.TemperatureEvent, model.TermopadInfo) error
}

// TermopadSvc репозиторий работы с термопадом. Держит постоянно подключение к термопаду.
//go:generate mockery --dir . --name TermopadSvc --output ./mocks
type TermopadSvc interface {
	// Ожидает очередное сообщение от текромпада и возвращает в своём результате полученные данные.
	EmmitTemperature() (*model.TermopadTemperatureEvent, error)
}