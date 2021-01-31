package config

import "time"

type (

	// Config конфигурация программы
	Config struct {

		// Описание логирования
		Log struct {

			// Путь к файлу лога
			Path string

			// Имя файал логирования
			Filename string `required:"true" default:"termopad.log"`

			// Уровень логирования
			Level string `required:"true" default:"warning"`

			// Выводить лог только на консоль
			Console bool `default:"false"`
		}

		// Описываем подключение к базе данных
		Db struct {

			// Тип базы данных (sqlite, mysql и т.п.)
			Type string `default:"sqlite"`

			// Путь к расположению базы данных
			Path string

			// Имя файла базы данных
			Filename string `required:"true" default:"termopad.sqlite"`

			// Колличество дней хранения ахрива замеров температуры в днях
			ArchiveDays int `default:"30"`

			// Период очистки архива до ArchiveDays в минутах
			CleanArchiveInterval int `default:"30"`
		}

		// Описание места хранения изображений с термопада
		Images struct {

			// Путь к корневой директории с изображениями
			Path string `default:"./imagedb/temperature"`
		}

		// Описание термопатодов
		Termopad struct {

			// Таймаут обращения к термопаду, когда он считается недоступным (в секундах)
			TimeoutAlive uint `default:"5"`

			// Таймаут потокогого опроса термопада (когда ожидаем изменения данных)
			Timeout uint `default:"1"`

			// Максималная нормальная температура
			MaxTemperature float64 `required:"true"`

			// Минимальная нормальная температура
			MinTemperature float64 `required:"true"`

			// Адреса термопадов
			Info []struct {

				// Идентификатор термопада. По нему будет сопоставляться база данных
				ID uint `required:"true"`

				// Идентификатор кабины для СУДОС
				Cabina uint

				// IP:Port адрес термопада
				Address string `required:"true" default:""`

				// Имя термопада
				Name string `required:"true" default:""`

				// Описание термопада
				Description string
			}
		}

		// Обслуживание WEB-сервера
		Http struct {

			// Порт WEB-сервера
			Port uint `required:"true" default:"8080"`

			// Корень директории со статическим контентом
			AssetsDir string `default:"assets"`

			// Максимальная длинна линнии текста при отображении в интерфейсе
			MaxLenghtLine int `default:"17"`

			// Заполненность термопадами страницы. Если реальных термопадов больше,
			// чем указано здесь - в конце их выведутся заглушки
			TermopadsOnPage int `default:"0"`

			// Максималная нормальная температура
			MaxTemperature float64 `required:"true"`

			// Минимальная нормальная температура
			MinTemperature float64 `required:"true"`
		}

		// Описание СУДОС
		Sudos struct {
			// Адрес WebSocket канала, например 127.0.0.1:8000/sudos
			Address string `requires:"true"`

			// Путь к папке и изображениями персон
			Path string `default:"./imagedb/persons"`
		}

		// Распознавание лица
		Recognize struct {
			// URL сервера распознавания
			URL string `requires:"true"`
			// Таймаут ожидания ответа (в милисекундах)
			TimeOut time.Duration `default:"1000"`
		}
	}
)
