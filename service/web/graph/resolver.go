package graph

import (
	"io/ioutil"
	"strconv"
	"sync"

	"github.com/kirsrus/termopad/server2/model"
	modelGraphQl "github.com/kirsrus/termopad/server2/service/web/graph/model"
	"github.com/kirsrus/termopad/server2/store"

	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
)

const (
	termopadsOnPage = 16
	maxTemperature  = 37.5
	minTemperature  = 34.0
)

// Описывает весь список термопадов
type termopads []model.TermopadInfo

// IssetID проверяет, что термопад с указанным id существует
func (m termopads) IssetID(id uint) bool {
	for _, v := range m {
		if v.ID == id {
			return true
		}
	}
	return false
}

// Resolver резолвер GraphQL. Инициируется NewResolver
type Resolver struct {
	log *logrus.Entry

	termopads termopads
	//termperatureEvent        chan model.TermopadTemperatureEvent
	temperatureSubscribePool        *sync.Map
	temperatureChangedSubscribePool *sync.Map
	temperatureUpdateSubscribePool  *sync.Map

	db store.DbStore

	termopadsOnPage uint
	maxTemperature  float64
	minTemperature  float64
}

// Конфигурация структуры Resolver
type ConfigResolver struct {
	Log *logrus.Logger

	TermopadsOnPage uint
	MaxTemperature  float64
	MinTemperature  float64
}

// NewResolver конструктор Resolver. Через termperatureEmit возвращается сигнал об измерении температуры
func NewResolver(termopads []model.TermopadInfo, db store.DbStore, config *ConfigResolver) (*Resolver, error) {
	if config == nil {
		return nil, errors.New("конфигурация не передана")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}
	if db == nil {
		return nil, errors.New("не передана база данных")
	}

	resolver := Resolver{
		log: config.Log.WithFields(map[string]interface{}{
			"module": "graphql",
			"scope":  "service",
		}),
		termopads: termopads,
		//termperatureEvent:        termperatureEmit,
		temperatureSubscribePool:        new(sync.Map),
		temperatureChangedSubscribePool: new(sync.Map),
		temperatureUpdateSubscribePool:  new(sync.Map),

		db: db,

		termopadsOnPage: termopadsOnPage,
		maxTemperature:  maxTemperature,
		minTemperature:  minTemperature,
	}
	if err := resolver.Configure(config); err != nil {
		return nil, errors.Trace(err)
	}

	return &resolver, nil
}

// Configure применение конфигурации к структрурк
func (r *Resolver) Configure(config *ConfigResolver) error {
	if config == nil {
		return errors.New("конфигурация не задана")
	}
	if config.TermopadsOnPage != 0 {
		r.termopadsOnPage = config.TermopadsOnPage
	}
	if config.MaxTemperature != 0 {
		r.maxTemperature = config.MaxTemperature
	}
	if config.MinTemperature != 0 {
		r.minTemperature = config.MinTemperature
	}
	return nil
}

// TemperatureChanged фиксация новой температуры
func (r Resolver) TemperatureChanged(temperature model.TemperatureChange) {
	r.temperatureSubscribePool.Range(func(key, value interface{}) bool {
		inChan, ok := value.(chan *modelGraphQl.Temperature)
		if !ok {
			r.log.Errorf("по каналу temperatureChangedSubscribePool пришёл неожиданный тип данных: %T, а должен быть %T", value, modelGraphQl.Temperature{})
			return true
		}

		select {
		case inChan <- &modelGraphQl.Temperature{
			ID:             strconv.Itoa(int(temperature.ID)),
			Job:            "set",
			Update:         temperature.CreateAt.Format("2006.01.02 15:04:05"),
			Temperature:    temperature.Temperature,
			Image:          &temperature.Image,
			Wigand:         strconv.Itoa(int(temperature.Wigand.ID)),
			WigandFasality: strconv.Itoa(int(temperature.Wigand.Fasality())),
			WigandNumber:   strconv.Itoa(int(temperature.Wigand.Number())),
			NameFirst:      &temperature.NameFirst,
			NameMiddle:     &temperature.NameMiddle,
			NameLast:       &temperature.NameLast,
			Organization:   &temperature.Organization,
			Departament:    &temperature.Departament,
			Postion:        &temperature.Postion,
		}:
			r.log.Debugf("данные отправлены на WEB")
		default:
			r.log.Warnf("канал %s из temperatureChangedSubscribePool переполнен", key)
			return true
		}
		return true
	})
}
