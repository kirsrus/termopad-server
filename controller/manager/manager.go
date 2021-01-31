package manager

import (
	"context"
	"io/ioutil"
	"math"
	"time"

	"github.com/kirsrus/termopad-server/controller"
	"github.com/kirsrus/termopad-server/model"
	"github.com/kirsrus/termopad-server/pkg/validator"
	"github.com/kirsrus/termopad-server/service"
	"github.com/kirsrus/termopad-server/store"

	"github.com/juju/errors"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	requestTimeout       = time.Second
	updatePersonInterval = 60 * time.Minute
	cleanBasePeriod      = time.Hour * 24 * 30
	cleanBaseInterval    = time.Minute * 30
)

// ConfigManager конфигурация Manager
type ConfigManager struct {
	Log *logrus.Logger

	TermopadCtl controller.TermopadCtl

	WebSvc   service.WebSvc
	SudosSvc service.SudosSvc
	DbStore  store.DbStore

	RequestTimeout       time.Duration
	UpdatePersonInterval time.Duration
	CleanBasePeriod      time.Duration
	CleanBaseInterval    time.Duration

	WebPort   uint
	AssetsDir string
}

// Manager основной менеджер работы со всеми сервисами. Инициируется через NewManage
type Manager struct {
	ctx       context.Context
	log       *logrus.Entry
	validator *validator.Validator

	termopadCtl controller.TermopadCtl

	webSvc   service.WebSvc
	sudosSvc service.SudosSvc
	dbStore  store.DbStore

	requestTimeout       time.Duration
	updatePersonInterval time.Duration
	cleanBasePeriod      time.Duration
	cleanBaseInterval    time.Duration

	e         *echo.Echo
	webPort   uint
	assetsDir string
}

// NewManager конструктор Manage
func NewManager(ctx context.Context, config *ConfigManager) (*Manager, error) {
	if config == nil {
		return nil, errors.New("не передана конфигурация")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}
	if config.TermopadCtl == nil {
		return nil, errors.New("не передан контроллер термопада")
	}
	if config.SudosSvc == nil {
		return nil, errors.New("не передан контроллер СУДОС")
	}
	if config.WebSvc == nil {
		return nil, errors.New("не передан сервис WEB")
	}
	if config.DbStore == nil {
		return nil, errors.New("не передан сервис базы данных")
	}

	manager := Manager{
		ctx: ctx,
		log: config.Log.WithFields(map[string]interface{}{
			"module": "manager",
			"scope":  "controller",
		}),
		validator:   validator.Get(),
		termopadCtl: config.TermopadCtl,
		sudosSvc:    config.SudosSvc,

		webSvc:  config.WebSvc,
		dbStore: config.DbStore,

		requestTimeout:       requestTimeout,
		updatePersonInterval: updatePersonInterval,
		cleanBasePeriod:      cleanBasePeriod,
		cleanBaseInterval:    cleanBaseInterval,

		e:         echo.New(),
		webPort:   80,
		assetsDir: "./assets/main",
	}
	if config.RequestTimeout != 0 {
		manager.requestTimeout = config.RequestTimeout
	}
	if config.UpdatePersonInterval != 0 {
		manager.updatePersonInterval = config.UpdatePersonInterval
	}
	if config.CleanBasePeriod != 0 {
		manager.cleanBasePeriod = config.CleanBasePeriod
	}
	if config.CleanBaseInterval != 0 {
		manager.cleanBaseInterval = config.CleanBaseInterval
	}
	if config.WebPort != 0 {
		manager.webPort = config.WebPort
	}
	if config.AssetsDir != "" {
		manager.assetsDir = config.AssetsDir
	}

	manager.configToLog()

	return &manager, nil
}

// Вывести значения конфигурациии в лог
func (m Manager) configToLog() {
	m.log.Debugf("requestTimeout: %s", m.requestTimeout)
	m.log.Debugf("updatePersonInterval: %s", m.updatePersonInterval)
	m.log.Debugf("cleanBasePeriod: %s", m.cleanBasePeriod)
	m.log.Debugf("cleanBaseInterval: %s", m.cleanBaseInterval)
	m.log.Debugf("webPort: %d", m.webPort)
	m.log.Debugf("assetsDir: %s", m.assetsDir)
}

// Serve начало процесса обработки поступающих данных
func (m Manager) Serve() error {
	done := make(chan error)
	termperature := make(chan *model.TermopadTemperatureEvent, 10)

	g := new(errgroup.Group)

	// Запуск контроллера обмена данных с термопадом
	g.Go(func() error {
		for {
			term, err := m.termopadCtl.EmmitTemperature()
			if err != nil {
				return err
			}
			select {
			case <-m.ctx.Done():
				return nil
			case termperature <- term:
			default:
				m.log.Warn("очередь termperature переполнена")
			}
		}
	})

	// Запуск хоускеппера для очистки базы данных от старых записей
	g.Go(func() error {
		for {
			// todo: временное решение пока не доведу до ума очистку базы.
			// Потом нужно будет перенести в нужно место с

			days := int(math.Round(m.cleanBaseInterval.Hours() / 24))
			err := m.dbStore.Clean(days)
			if err != nil {
				return errors.Trace(err)
			}
			time.Sleep(m.cleanBaseInterval)
		}
	})

	go func() {
		err := g.Wait()
		done <- errors.Trace(err)
	}()

	// Обработка полученных от термопадов данных
	for {
		select {
		case err := <-done:
			return errors.Trace(err)
		case <-m.ctx.Done():
			return nil
		case temp := <-termperature:
			go func() {
				m.temperatureInWorker(temp)
			}()
		}
	}
}

// Обработчик пришедшей с термопада температуры
func (m Manager) temperatureInWorker(temp *model.TermopadTemperatureEvent) {
	g := new(errgroup.Group)
	found := true
	// Пытаемся получить данные из локальной БД. Если информации о персоне нет или данные
	// устарели, посылаем запрос в СУДОС для корректировки.
	person, err := m.dbStore.GetPerson(temp.Temperature.Wigand.ID)
	if err != nil {
		if m.dbStore.IsNotFound(err) {
			found = false
		} else {
			m.log.Error(err)
			return
		}
	}

	// Персона обнаружена
	if found {
		m.log.Debugf("данные о %d получены из БД", temp.Temperature.Wigand.ID)

		m.webSvc.TemperatureChanged(model.TemperatureChange{
			ID:           temp.Info.ID,
			CreateAt:     *temp.CreateAt,
			Temperature:  math.Round(temp.Temperature.Temperature*10) / 10,
			Image:        temp.Image,
			Wigand:       temp.Temperature.Wigand,
			NameFirst:    person.Name,
			NameMiddle:   person.MiddleName,
			NameLast:     person.Family,
			Organization: person.Organization,
			Departament:  person.Department,
			Postion:      person.Position,
		})

		go func() {
			err := m.sudosSvc.SetPersonTemperature(*person, temp.Temperature, temp.Info)
			if err != nil {
				m.log.Warn(err)
			}
		}()

		// Если данные устарели, запрашиваем у СУДОС более новые данные
		if time.Since(*person.UpdateAt) > m.updatePersonInterval {
			m.log.Debugf("запрос у СУДОС о %d т.к. прошло много времени", temp.Temperature.Wigand.ID)
			g.Go(func() error {
				person, err := m.sudosSvc.Person(temp.Temperature.Wigand)
				if err != nil {
					m.log.Warn(err)
					return errors.Trace(err)
				}
				// Сохраняем полученное от СУДОС изображение в БД
				if len(person.Image) != 0 {
					if err := m.dbStore.SetPersonImage(person.Wigand.ID, person.Image); err != nil {
						m.log.Warnf("ошибка сохранения изображения в БД: %v", err)
					}
				}
				// todo: кастыль от ошибки валидатора: panic: reflect.Value.Convert: value of type string cannot be converted to type uint8
				// Валидатор затыкается на BASE64 изображении (возможно, гдето у них превышается предел длинны)
				person.Image = make([]byte, 0)

				// Сохраняем данные о полученной персоны в БД
				_, update, err := m.dbStore.SetPerson(*person)
				if err != nil {
					m.log.Error(err)
					return errors.Trace(err)
				}
				if update {
					m.log.Debugf("в связи с истечением срока годности обновлена запись для wigand=%s (%s)", temp.Temperature.Wigand, person.Family)
				}
				return nil
			})
		}
	} else {
		// Персона не обнаружена

		m.webSvc.TemperatureChanged(model.TemperatureChange{
			ID:          temp.Info.ID,
			CreateAt:    *temp.CreateAt,
			Temperature: math.Round(temp.Temperature.Temperature*10) / 10,
			Image:       temp.Image,
			Wigand:      temp.Temperature.Wigand,
		})

		g.Go(func() error {
			person, err := m.sudosSvc.Person(temp.Temperature.Wigand)
			if err != nil {
				return errors.Trace(err)
			}

			// Сохраняем изображение персоны, полученой от СУДОС в БД
			if len(person.Image) != 0 {
				if err = m.dbStore.SetPersonImage(person.Wigand.ID, person.Image); err != nil {
					m.log.Warnf("ошибка сохранения изображения персоны в БД: %v", err)
				}
			}
			// todo: кастыль от ошибки валидатора: panic: reflect.Value.Convert: value of type string cannot be converted to type uint8
			// Валидатор затыкается на BASE64 изображении (возможно, гдето у них превышается предел длинны)
			person.Image = make([]byte, 0)

			// Сохраняем данные о персоне в локальную БД
			_, _, err = m.dbStore.SetPerson(*person)
			if err != nil {
				m.log.Error(err)
				return errors.Trace(err)
			}

			m.webSvc.TemperatureChanged(model.TemperatureChange{
				ID:          temp.Info.ID,
				CreateAt:    *temp.CreateAt,
				Temperature: math.Round(temp.Temperature.Temperature*10) / 10,
				Image:       temp.Image,
				Wigand:      temp.Temperature.Wigand,
				NameFirst:   person.Name,
				NameMiddle:  person.MiddleName,
				NameLast:    person.Family,
				Departament: person.Department,
				Postion:     person.Position,
			})

			go func() {
				err := m.sudosSvc.SetPersonTemperature(*person, temp.Temperature, temp.Info)
				if err != nil {
					m.log.Warn(err)
				}
			}()

			return nil
		})
	}

	// Записываем температуру в локальную БД. Делаем секцию не критичной, только в лог, чтобы
	// не портить весь процесс, если он не логируется.
	err = m.dbStore.SetTemperatureLog(temp.Info.ID, temp.Temperature.Wigand.ID, temp.Temperature.Temperature, temp.Image)
	if err != nil {
		m.log.Error(err)
	}

	_ = g.Wait()
}
