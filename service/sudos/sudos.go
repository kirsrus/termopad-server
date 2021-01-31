package sudos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/kirsrus/termopad-server/model"
	"github.com/kirsrus/termopad-server/pkg/validator"
	"github.com/kirsrus/termopad-server/service"

	"github.com/gorilla/websocket"
	"github.com/juju/errors"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	readChanCapacity     = 10
	writeChanCapacity    = 10
	cacheExpiration      = 5 * time.Minute  // Время жизни записи в кэше
	cacheCleanupInterval = 10 * time.Minute // Интервал очистки мёртвых записях (сборщик мусора)
	reconnectTimeout     = 10 * time.Second
	requestTimeout       = 3 * time.Second
	templateNormal       = "температура в норме (%0.1f°)"
	templateAlarm        = "температура повышенная (%0.1f°)"
	templateLower        = "низкая температура (%0.1f°)"
	maxTemperature       = 37.5
	minTemperature       = 34.0
)

// Тип текущего состояния подключения к термопаду
type conectType int

const (
	connectUnknown = iota
	connectSuccess
	connectFailed
)

// Sudos общение с СУДОС. Имплементирует интрерфейс SudosSvc. Инициируется конструктором NewSudos
type Sudos struct {
	ctx              context.Context
	log              *logrus.Entry
	sudosUrl         string
	reconnectTimeout time.Duration
	requestTimeout   time.Duration
	connectedFlag    conectType
	readChan         chan []byte // Канал получения данных от СУДОС
	writeChan        chan []byte // Канал отправки данных в СУДОС
	cache            *cache.Cache
	templateNormal   string
	templateAlarm    string
	templateLower    string
	maxTemperature   float64
	minTemperature   float64
	validator        *validator.Validator
}

// ConfigSudos конфигурация конструктора NewSudos
type ConfigSudos struct {
	Log              *logrus.Logger
	SudosUrl         string `conform:"trim" validate:"required,websocket"`
	ReconnectTimeout time.Duration
	RequestTimeout   time.Duration
	TemplateNormal   string
	TemplateAlarm    string
	TemplateLower    string
	MaxTemperature   float64
	MinTemperature   float64
}

// NewSudos констурктор Sudos
func NewSudos(ctx context.Context, config *ConfigSudos) (service.SudosSvc, error) {
	if config == nil {
		return nil, errors.New("не задана конфигурация config")
	} else if err := validator.Get().ValidateWithConform(config); err != nil {
		return nil, errors.Annotate(err, "ошибка в конфигурации")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}

	sudos := &Sudos{
		ctx: ctx,
		log: config.Log.WithFields(map[string]interface{}{
			"module":  "sudos",
			"scope":   "store",
			"address": config.SudosUrl,
		}),
		sudosUrl:         config.SudosUrl,
		reconnectTimeout: reconnectTimeout,
		requestTimeout:   requestTimeout,
		connectedFlag:    connectUnknown,
		readChan:         make(chan []byte, readChanCapacity),
		writeChan:        make(chan []byte, writeChanCapacity),
		cache:            cache.New(cacheExpiration, cacheCleanupInterval),
		templateNormal:   templateNormal,
		templateAlarm:    templateAlarm,
		templateLower:    templateLower,
		maxTemperature:   maxTemperature,
		minTemperature:   minTemperature,
		validator:        validator.Get(),
	}
	if config.ReconnectTimeout != 0 {
		sudos.reconnectTimeout = config.ReconnectTimeout
	}
	if config.RequestTimeout != 0 {
		sudos.requestTimeout = config.RequestTimeout
	}

	go sudos.loop()

	return sudos, nil
}

// Кольцевое обращение к СУДОС
func (m *Sudos) loop() {
	m.log.Info("старт работы модуля")
	for {
		select {
		case <-m.ctx.Done():
			m.log.Info("завершение работы модуля")
			return
		default:
		}

		err := m.connect()
		if err != nil && err.Error() != context.Canceled.Error() {
			time.Sleep(m.reconnectTimeout)
		}
	}
}

// Подключение по WebSocket к СУДОС
func (m *Sudos) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(m.sudosUrl, nil)
	if err != nil {
		if m.connectedFlag == connectUnknown || m.connectedFlag == connectSuccess {
			m.log.Warnf("ошибка подключения: %v", err)
		}
		m.connectedFlag = connectFailed
		return errors.Trace(err)
	}
	defer func() { _ = conn.Close() }()
	if m.connectedFlag == connectUnknown || m.connectedFlag == connectFailed {
		m.log.Infof("подключение установлено")
		m.connectedFlag = connectSuccess
	}

	g := new(errgroup.Group)

	// Чтение из канала
	g.Go(func() error {
		for {
			tpe, message, err := conn.ReadMessage()
			if err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					m.log.Warnf("ошибка чтения из WebSocket: %v", err)
					return errors.Trace(err)
				}
				return nil
			}
			if tpe != websocket.TextMessage {
				m.log.Warnf("пропущено нетиповое послание типа %d, размера %d", tpe, len(message))
				continue
			}

			// TODO используется только для тестов. Потом удалить
			// Логируем отверт от СУДОСА, чтобы подсавлять их для тестера
			//if false {
			//	const sudosLog = "sudos.response.log"
			//	if _, err = os.Stat(sudosLog); os.IsNotExist(err) {
			//		if file, err := os.Create(sudosLog); err == nil {
			//			if _, err = file.WriteString(string(message) + "\n"); err == nil {
			//				_ = file.Close()
			//			}
			//		}
			//	} else if err == nil {
			//		if file, err := os.OpenFile(sudosLog, os.O_APPEND, os.ModePerm); err == nil {
			//			if _, err = file.WriteString(string(message) + "\n"); err == nil {
			//				_ = file.Close()
			//			}
			//		}
			//	} else {
			//		m.log.Error(err)
			//	}
			//}

			// Учитываем, что это только ответ на прошлый запрос
			var person model.SudosPersonResponse
			if err = json.Unmarshal(message, &person); err != nil {
				m.log.Errorf("не удалось распаковать JSON от СУДОС: %v", err)
				continue
			}
			if err = m.validator.ValidateWithConform(&person); err != nil {
				m.log.Errorf("ошибка валидации данных о персоне от СУДОС: %v", err)
				continue
			}

			// Ищем в кэше, кому направлен этот ответ
			if value, found := m.cache.Get(person.UidRequest); found {
				channels := value.([]chan model.SudosPersonResponse)
				for idx := range channels {
					select {
					case channels[idx] <- person:
					default:
						m.log.Warnf("очередь readChan[%d] для %s переполнена", idx, person.UidRequest)
					}
				}
				m.log.Debugf("удаляем данные для ключа %s", person.UidRequest)
				m.cache.Delete(person.UidRequest)
			}

			select {
			case <-m.ctx.Done():
				return m.ctx.Err()
			default:
			}
		}
	})

	// Запись в канал
	g.Go(func() error {
		for write := range m.writeChan {
			err = conn.WriteMessage(websocket.TextMessage, write)
			if err != nil {
				m.log.Warnf("ошибка записи в WebSocket: %v", err)
				return errors.Trace(err)
			}
		}
		return nil
	})

	err = g.Wait()
	return errors.Trace(err)
}

// Person запрашивает даныне персоны по номеру Wigand.
func (m Sudos) Person(wigand model.Wigand) (*model.Person, error) {
	if err := validator.Get().ValidateWithConform(&wigand); err != nil {
		return nil, errors.Annotate(err, "некорретный параметр wigand")
	}
	response := make(chan model.SudosPersonResponse, 1)
	key := strconv.Itoa(int(wigand.ID))

	// Если запрос на такой Wignad уже есть, просто добавляем канал ответа к
	// существующему каналу, чтобы ответ был им обоим. Иначе создаём новую запись
	if value, found := m.cache.Get(key); found {
		channels := value.([]chan model.SudosPersonResponse)
		channels = append(channels, response)
		m.cache.Set(key, channels, cache.DefaultExpiration)
		m.log.Debugf("обновлили в кэше ключ %s", key)
	} else {
		channels := make([]chan model.SudosPersonResponse, 0)
		channels = append(channels, response)
		err := m.cache.Add(key, channels, cache.DefaultExpiration)
		if err != nil {
			return nil, errors.Annotate(err, "ошибка добавления в кэш")
		}
		m.log.Debugf("добавили в кэш ключ %s", key)
	}

	request := model.SudosPersonRequest{
		UidRequest:  key,
		Facility:    wigand.Fasality(),
		Numer:       wigand.Number(),
		Message:     "",
		AlarmStatus: false,
	}
	requestByte, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Annotate(err, "ошибка кодирования SudosPersonRequest в JSON")
	}
	select {
	case m.writeChan <- requestByte:
	default:
		m.log.Warn("канал передачи данных writeChan забит")
		return nil, errors.New("канал передачи данных забит")
	}

	// Ожидаем ответа
	select {
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	case <-time.After(m.requestTimeout):
		m.log.Debugf("время ожидания ответа для ключа %s вышло", key)
		return nil, errors.Errorf("время ожидания ответа для ключа %s вышло", key)
	case resp := <-response:
		// Декодирование изображения от СУДОС
		image := make([]byte, 0)
		if resp.Photo != "" {
			imageByt, err := base64.StdEncoding.DecodeString(resp.Photo)
			if err != nil {
				m.log.Warnf("ошибка декодирование изображения от СУДОС: %v", err)
			} else {
				image = imageByt
			}
		}
		person := model.Person{
			Wigand:       wigand,
			Family:       resp.Family,
			Name:         resp.Name,
			MiddleName:   resp.Patronymic,
			Organization: resp.Contora,
			Department:   resp.SubOtdel,
			Position:     resp.Profy,
			Image:        image,
		}
		return &person, nil
	}
}

// SetPersonTemperature устанавливает температуру персоны.
func (m Sudos) SetPersonTemperature(person model.Person, temperature model.TemperatureEvent, termopad model.TermopadInfo) error {
	if err := m.validator.Validate(&person.Wigand); err != nil {
		return errors.New("ошибка описсания wigand: " + err.Error())
	}

	// Опредеяем результирующую строку
	var message string
	var alarm bool
	switch true {
	case temperature.Temperature >= m.maxTemperature:
		message = fmt.Sprintf(m.templateAlarm, temperature.Temperature)
		alarm = true
	case temperature.Temperature < m.minTemperature:
		message = fmt.Sprintf(m.templateLower, temperature.Temperature)
	default:
		message = fmt.Sprintf(m.templateNormal, temperature.Temperature)
	}

	// Отправляем результат
	request := model.SudosPersonRequest{
		UidRequest:  "",
		Facility:    person.Wigand.Fasality(),
		Numer:       person.Wigand.Number(),
		Message:     message,
		AlarmStatus: alarm,
		Cabina:      termopad.SudosID,
	}
	msg, err := json.Marshal(&request)
	if err != nil {
		return errors.Annotate(err, "ошибка создания JSON")
	}
	m.log.Debugf("отсыл результирующей температуры в СУДОС: %s", message)

	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	case m.writeChan <- msg:
	//case <-time.After(m.requestTimeout):
	//	return errors.New("таймаут ожидания отправки")
	default:
		m.log.Warnf("канал writeChan преполнен")
	}
	return nil
}
