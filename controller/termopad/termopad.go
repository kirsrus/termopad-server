package termopad

import (
	"context"
	"io/ioutil"

	"github.com/kirsrus/termopad/server2/model"
	"github.com/kirsrus/termopad/server2/service"
	"github.com/kirsrus/termopad/server2/store"

	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	// Величина канала информации от термопадов
	eventCapacity = 10
)

// Termopad контроллер упдавления группой термопадов. Инициализируестя через NewTermopad. Держит постоянное
// подключение ко всем термопадам. Через Start возвращает канал получения значений со всех термопадов.
// Через Restart можно изменить список термопадов, с которых данные получаются.
type Termopad struct {
	ctx context.Context
	log *logrus.Entry

	termopadsSvc []service.TermopadSvc
	dbStore      store.DbStore

	event chan *model.TermopadTemperatureEvent
	stop  chan error

	// Величина канала информации от термопадов
	eventCapacity uint
}

// ConfigTermopad конфигурация Termopad
type ConfigTermopad struct {
	Log *logrus.Logger
	// Величина канала информации от термопадов
	EventCapacity uint
}

// NewTermopad конструтор Termopad
func NewTermopad(ctx context.Context, termopadsSvc []service.TermopadSvc, dbStore store.DbStore, config *ConfigTermopad) (*Termopad, error) {
	if config == nil {
		return nil, errors.New("не установлен config")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}
	if termopadsSvc == nil {
		return nil, errors.New("не указан список termopadsSvc")
	}
	if dbStore == nil {
		return nil, errors.New("не указана служба dbStore")
	}

	termopad := Termopad{
		ctx: ctx,
		log: config.Log.WithFields(map[string]interface{}{
			"module": "termopad",
			"scope":  "controller",
		}),
		termopadsSvc: termopadsSvc,
		dbStore:      dbStore,

		event: make(chan *model.TermopadTemperatureEvent, eventCapacity),
		stop:  make(chan error),

		eventCapacity: eventCapacity,
	}
	if config.EventCapacity != 0 {
		termopad.eventCapacity = config.EventCapacity
	}
	go termopad.loop()

	return &termopad, nil
}

// Бесконечное получение данных со всех термопадов
func (m Termopad) loop() {
	m.log.Info("старт работы модуля")
	for {
		g := new(errgroup.Group)

		for _, v := range m.termopadsSvc {
			v := v
			g.Go(func() error {
				for {
					event, err := v.EmmitTemperature()
					if err != nil {
						return errors.Trace(err)
					}
					m.event <- event
				}
			})
		}

		err := g.Wait()
		if err != nil && err.Error() != context.Canceled.Error() {
			m.log.Error(err)
		}
		select {
		case <-m.ctx.Done():
			m.log.Info("завершение работы модуля")
			return
		}
	}
}

// EmmitTemperature ожидает события поступление на любой из термопадов события
// о текущей термпературе. Возвращает context.Cacnel при принудиельно завершении работы
func (m Termopad) EmmitTemperature() (*model.TermopadTemperatureEvent, error) {
	select {
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	case temp := <-m.event:
		// Сохраняем поученное изображение в БД
		imgName, err := m.dbStore.SetTempImage(*temp.CreateAt, temp.Temperature.Wigand, temp.Temperature.Image)
		if err != nil {
			return nil, errors.Trace(err)
		}
		temp.Image = *imgName
		return temp, nil
	}
}
