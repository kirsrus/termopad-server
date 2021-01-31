package model

// SudosPersonRequest запрос в СУДОС на информацию о пресоне и установки температуры
type SudosPersonRequest struct {
	// Токен
	UidRequest string `json:"uid_request"`

	// Фасалити виганд
	Facility uint `json:"facility"`

	// Номер виганд
	Numer uint `json:"numer"`

	// Сообщение, которое нужно записать в СКУД. Если пустое, то это запрос на
	// информацию о человеке
	Message string `json:"mess_skud"`

	// Является ли температура человека тревожной (т.к. превышает норму). Поле
	// действует только в связке с Massage
	AlarmStatus bool `json:"alarm"`

	// Номер кабины
	Cabina uint  `json:"cabina"`
}

// SudosPersonResponse ответ от СУДОС
type SudosPersonResponse struct {
	// Токен
	UidRequest string `json:"uid_request" conform:"trim"`
	// IP-адрес ведущего сервера
	IpVedushiy string `json:"ip_vedushiy" conform:"trim"`
	// Номер записи
	NumRec uint `json:"num_rec"`
	// Фамилия
	Family string `json:"family" conform:"trim" validate:"required"`
	// Имя
	Name string `json:"name" conform:"trim" validate:"required"`
	// Отчество
	Patronymic string `json:"patronymic" conform:"trim"`
	// Номер карты
	Numer uint `json:"numer"`
	// Организация
	Contora string `json:"contora" conform:"trim"`
	// Отдел
	Otdel string `json:"otdel" conform:"trim"`
	// Цех
	SubOtdel string `json:"sub_otdel" conform:"trim"`
	// Должность
	Profy string `json:"profy" conform:"trim"`
	// Тип пропуска
	Type string `json:"type" conform:"trim"`
	// Имя файла фото
	Photo string `json:"photo" conform:"trim"`
}
