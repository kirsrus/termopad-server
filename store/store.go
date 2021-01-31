package store

import (
	"time"

	"github.com/kirsrus/termopad/server2/model"
)

// DbStore репозирторий общения с БД
//go:generate mockery --dir . --name DbStore --output ./mocks
type DbStore interface {
	// Проверяет, что ошибка err обозначает, что записи не найдены
	IsNotFound(err error) bool

	// Получает персону из БД по номеру wigand. Отсутсвие персоны в БД проверяется через IsNotFound
	GetPerson(wigandID uint) (*model.Person, error)

	// Добавляет персону в БД. Если персоны нет, она будет добавлена и вернётся true.
	// Если персона уже была, она будет обновлена и вернётся false
	SetPerson(model.Person) (*model.Person, bool, error)

	// Получает изображение по его идентификационнай имени в БД
	TempImage(string) ([]byte, error)
	// Сохраняет изображение в БД и возвращает его идентификационное имя файла
	SetTempImage(time.Time, model.Wigand, []byte) (*string, error)

	// Возвращает путь до изображения персоны
	PersonImage(wigand uint) ([]byte, error)
	// Сохраняет изображения персоны в БД и возвращает имя созданного файла
	SetPersonImage(wigand uint, content []byte) error

	// Получение лога температур для указаной персоны, за период, не более указанного
	//TemperatureLogByPerson(uint, time.Duration) ([]model.TemperatureLog, error)
	TemperatureLogByPerson(uint, time.Duration) ([]TemperatureLog, error)
	// Получение лога температур по выбранному термопаду, за период, не более указанного
	TemperatureLogByTermopad(uint, time.Duration) ([]TemperatureLog, error)
	// Сохранение текущего замера температуры в лог замеров
	SetTemperatureLog(termopadID uint, wigandID uint, temperature float64, imageName string) error
	// Возвращает описание последней замерившейся персоны и её температуры на термопаде.
	// Если запись не найдена или не найдена персона для этой записи, возвращается gorm.ErrRecordNotFound
	LastPerson(termopadID uint) (*LastPerson, error)

	// Возвращает значения температур для персоны с wigandID за days дней (со смещением offsetDays) по
	// каждому замеру температуры. Если compact=true - данные замеров сжимаются до дней и температура показыватся
	// только минимальная и максимальная для каждого дня
	PersonLog(wigandID uint, days uint, offsetDays uint, compact bool) ([]model.TemperatureMetric, error)

	// Возвращает значения температур для термопада с termopadID за days дней (со смещением offsetDays) по
	// каждому замеру температуры. Если compact=true - данные замеров сжимаются до дней и температура показыватся
	// только минимальная и максимальная для каждого дня
	TermopadLog(termopadID uint, days uint, offsetDays uint, compact bool) ([]model.TemperatureMetric, error)

	// Очищает записи в БД старше days дней
	Clean(days int) error
}

// TemperatureLog описывает данные из лога температуры
type TemperatureLog struct {
	// Идентификатор записи в БД
	ID         int
	TermopadID int
	CreatedAt  *time.Time
	Person     model.Person
}

// LastPerson информация о последней зарегистрированной на термопаде персоны
type LastPerson struct {
	CreatedAt    *time.Time
	Termperature model.Temperature
	Person       model.Person
}
