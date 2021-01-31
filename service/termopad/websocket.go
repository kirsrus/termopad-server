package termopad

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kirsrus/termopad-server/model"
	"github.com/kirsrus/termopad-server/pkg/validator"
	"github.com/kirsrus/termopad-server/service"

	"github.com/gorilla/websocket"
	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
)

const (
	// Шаблон доступа к отснятому изображению с температурой
	ImageUrlTemplate  = "http://%s/static/img/orig/%s"
	MaximumResultChan = 20
	ReconnectTimeout  = 5 * time.Second
	DownloadTimeout   = 2 * time.Second
)

// Тип текущего состояния подключения к термопаду
type connectType int

const (
	connectUnknown = iota
	connectSuccess
	connectFailed
)

// Websocket имплементация подключения к термопаду по WebSocket. Инициируется через NewWebsocket.
// Постоянно держит соединение, пока существует класс.
type Websocket struct {
	termopadInfo     model.TermopadInfo
	ctx              context.Context
	log              *logrus.Entry
	reconnectTimeout time.Duration
	downloadTimeout  time.Duration
	// Канал передачи результата
	resultChan    chan model.TemperatureEvent
	connectedFlag connectType
}

// ConfigWebsocket конфигурация Websocket
type ConfigWebsocket struct {
	Log              *logrus.Logger
	TermopadInfo     model.TermopadInfo
	ReconnectTimeout time.Duration
	DownloadTimeout  time.Duration
}

// NewWebsocket конструктор структуры Websocket
func NewWebsocket(ctx context.Context, config *ConfigWebsocket) (service.TermopadSvc, error) {
	var err error
	valid := validator.Get()
	if config == nil {
		return nil, errors.New("не задана конфигурация config")
	}
	if err = valid.Validate(&config.TermopadInfo); err != nil {
		return nil, errors.Annotate(err, "некорректное описание термопада")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}

	res := &Websocket{
		termopadInfo: config.TermopadInfo,
		ctx:          ctx,
		log: config.Log.WithFields(map[string]interface{}{
			"module":  "termopad",
			"scope":   "store",
			"id":      config.TermopadInfo.ID,
			"address": config.TermopadInfo.URL,
		}),
		reconnectTimeout: ReconnectTimeout,
		downloadTimeout:  DownloadTimeout,
		resultChan:       make(chan model.TemperatureEvent, MaximumResultChan),
		connectedFlag:    connectUnknown,
	}
	if config.ReconnectTimeout != 0 {
		res.reconnectTimeout = config.ReconnectTimeout
	}
	if config.DownloadTimeout != 0 {
		res.downloadTimeout = config.DownloadTimeout
	}

	// Запускаем бесконечный цикл переподключения к термопаду.
	go res.loop()

	return res, nil
}

// Бесконечный цикл обращения к WebSocket термопада. При завершении работы черезе context.Cancel просто
// завершаем его обработку
func (m *Websocket) loop() {
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

// Подключение по WebSocket к термопаду
func (m *Websocket) connect() error {
	read := make(chan []byte, 10)
	done := make(chan error)

	conn, _, err := websocket.DefaultDialer.Dial(m.termopadInfo.URL, nil)
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

	// Бесконечно читаем из канала WebSocket
	go func() {
		for {
			tpe, message, err := conn.ReadMessage()
			if err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					m.log.Warnf("ошибка чтения из WebSocket: %v", err)
					done <- errors.Trace(err)
				} else {
					done <- nil
				}
				return
			}
			if tpe != websocket.TextMessage {
				m.log.Warnf("пропущено нетиповое послание типа %d, размера %d", tpe, len(message))
				continue
			}

			select {
			case <-m.ctx.Done():
				return
			case read <- message:
			default:
				m.log.Warnf("очередь read переполнена")
			}
		}
	}()

	// Имя предыдущего скачанного изображения, чтобы не попадать на
	// ошибку термопада с одновременно несколькими одинаковыми сообщениями о скачивании в течении
	// нескльких миллисекунд с одними и темиже данными, но разным временем посыла
	var prevousFileName string
	// Обрабатываем результат чтения
	for {
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()
		case err := <-done:
			return err
		case message := <-read:
			msg := model.TermopadAction{}
			if err = json.Unmarshal(message, &msg); err != nil {
				m.log.Warnf("пршиёл некорректный json \"%s\" с ошибкой: %s", string(message), err.Error())
				continue
			}
			if err = msg.Validate(); err != nil {
				m.log.Warnf("ошибка валидации полученного json: %v", err)
				continue
			}

			if msg.Action == "newImage" && msg.FileName != prevousFileName {
				prevousFileName = msg.FileName

				// Распарсиваем имя файла (там все данные)
				termopadFileName := model.TermopadFileName{}
				if err = termopadFileName.Parse(msg.FileName); err != nil {
					m.log.Warnf("нераспознаваемое имя файла '%s': %v", msg.FileName, err)
					continue
				}

				// Скачиваем изображение
				addr, _ := url.Parse(m.termopadInfo.URL)
				URL := fmt.Sprintf(ImageUrlTemplate, addr.Host, msg.FileName)
				immageContent, err := m.downloadContent(URL)
				if err != nil {
					continue
				}

				res := model.TemperatureEvent{
					Temperature: termopadFileName.Temperature,
					Wigand:      termopadFileName.Wigand,
					Image:       immageContent,
				}

				select {
				case m.resultChan <- res:
				default:
					m.log.Warnf("канал resultChan переполнен")
					continue
				}
			}
		}
	}
}

// Скачиваем контент по URL адресу
func (m Websocket) downloadContent(URL string) ([]byte, error) {
	var client = &http.Client{
		Timeout: m.downloadTimeout, // Устанавливаем таймаут обращения
	}

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return []byte{}, errors.Trace(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		m.log.Warnf("сообщение %s не скачано: %v", req.URL.String(), err)
		return []byte{}, errors.Trace(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("для скачивания %s возвращён статус %d", URL, resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		m.log.Warnf("ошибка получения тела изображения %s: %s", req.URL.String(), err)
		return []byte{}, errors.Trace(err)
	}

	m.log.Debugf("изображение %s (%d байт) скачано", req.URL.String(), len(data))
	return data, nil
}

// EmmitTemperature ожидает данные от термопада и возвращает в свойм результате полученные данные.
// В случае штатного завершения работы, возвращаетя ошибка context.Canceled
func (m Websocket) EmmitTemperature() (*model.TermopadTemperatureEvent, error) {
	select {
	case result := <-m.resultChan:
		t := time.Now()
		res := model.TermopadTemperatureEvent{
			CreateAt:    &t,
			Info:        m.termopadInfo,
			Temperature: result,
		}
		return &res, nil
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	}
}
