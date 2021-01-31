package db

import (
	"context"
	"fmt"
	"github.com/patrickmn/go-cache"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/kirsrus/termopad-server/model"
	"github.com/kirsrus/termopad-server/pkg/config"
	"github.com/kirsrus/termopad-server/pkg/tool"
	"github.com/kirsrus/termopad-server/pkg/validator"
	"github.com/kirsrus/termopad-server/store"

	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const (
	cacheDuration = 10 * time.Minute
	cacheCleared  = time.Hour
)

// Db обращение к базе данных. Инициируется через NewDb
type Db struct {
	ctx                context.Context
	log                *logrus.Entry
	db                 *gorm.DB
	validator          *validator.Validator
	RootTemperatureDir string
	RootPersonDir      string
	globalConfig       *config.Config

	personCache *cache.Cache
}

// ConfigDb конфигурацияи класса NewDb
type ConfigDb struct {
	Log                *logrus.Logger
	DbFile             string
	RootTemperatureDir string
	RootPersonDir      string
	GlobalConfig       *config.Config
}

// NewDb конструктор класса Db
func NewDb(ctx context.Context, config *ConfigDb) (store.DbStore, error) {
	if config == nil {
		return nil, errors.New("не указана конфигурация")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}
	if config.DbFile == "" {
		return nil, errors.New("в конфигурациине указана строка подлкючения")
	}
	if config.GlobalConfig == nil {
		return nil, errors.New("не передана глобальная конфигурация программы")
	}

	// Подключаемся к БД и запускаем миграции
	conn, err := gorm.Open(sqlite.Open(config.DbFile), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		return nil, errors.Annotate(err, "ошибка подключения к файлу БД")
	}
	err = conn.AutoMigrate(Config{}, Person{}, Termopad{}, Temperature{})
	if err != nil {
		return nil, errors.Annotate(err, "ошибка миграции БД")
	}

	db := Db{
		ctx: ctx,
		log: config.Log.WithFields(map[string]interface{}{
			"module": "db",
			"scope":  "store",
		}),
		validator:          validator.Get(),
		db:                 conn,
		RootTemperatureDir: config.GlobalConfig.Images.Path,
		RootPersonDir:      config.GlobalConfig.Sudos.Path,
		globalConfig:       config.GlobalConfig,

		personCache: cache.New(cacheDuration, cacheCleared),
	}
	if config.RootTemperatureDir != "" {
		db.RootTemperatureDir = config.RootTemperatureDir
	}

	return &db, nil
}

// IsNotFound проверяет, что ошибка err обозначает, что записи не найдены
func (m Db) IsNotFound(err error) bool {
	return err.Error() == gorm.ErrRecordNotFound.Error()
}

// GetPerson получает персону из БД по номеру wigand. Отсутсвие персоны в БД проверяется через IsNotFound
func (m Db) GetPerson(wigandID uint) (*model.Person, error) {
	if wigandID == 0 {
		return nil, errors.New("передан некорректный номер wigand=0")
	}

	var person Person
	err := m.db.Where("wigand = ?", wigandID).Take(&person).Error
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, errors.Trace(err)
	}

	res := model.Person{
		CreateAt:     &person.CreatedAt,
		UpdateAt:     &person.UpdatedAt,
		Wigand:       model.Wigand{ID: uint(person.Wigand)},
		Family:       person.Family,
		Name:         person.Name,
		MiddleName:   person.MiddleName,
		Organization: person.Organization,
		Department:   person.Department,
		Position:     person.Position,
	}
	return &res, nil
}

// SetPerson добавляет персону в БД. Если персоны нет, она будет добавлена и вернётся true.
// Если персона уже была, она будет обновлена и вернётся false
func (m Db) SetPerson(person model.Person) (*model.Person, bool, error) {
	if err := m.validator.Validate(&person); err != nil {
		return nil, false, errors.Annotate(err, "ошибка валидации")
	}

	var isPerson Person
	err := m.db.Where("wigand = ?", person.Wigand.ID).Take(&isPerson).Error
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, false, errors.Trace(err)
	}
	newPerson := Person{}
	newPerson.FromPerson(person)
	if err != nil {
		// Добавляем новую запись
		err := m.db.Create(&newPerson).Error
		if err != nil {
			return nil, false, errors.Annotate(err, "ошибка добавления в БД")
		}
		res := model.Person{
			CreateAt:     &newPerson.CreatedAt,
			UpdateAt:     &newPerson.UpdatedAt,
			Wigand:       model.Wigand{ID: uint(newPerson.Wigand)},
			Family:       newPerson.Family,
			Name:         newPerson.Name,
			MiddleName:   newPerson.MiddleName,
			Organization: newPerson.Organization,
			Department:   newPerson.Department,
			Position:     newPerson.Position,
		}
		return &res, true, nil
	} else {
		// Обновляем существующую
		isPerson.Update(newPerson)
		err := m.db.Model(&isPerson).Updates(isPerson).Error
		if err != nil {
			return nil, false, errors.Annotate(err, "ошибка обновления записи")
		}
		res := model.Person{
			CreateAt:     &isPerson.CreatedAt,
			UpdateAt:     &isPerson.UpdatedAt,
			Wigand:       model.Wigand{ID: uint(isPerson.Wigand)},
			Family:       isPerson.Family,
			Name:         isPerson.Name,
			MiddleName:   isPerson.MiddleName,
			Organization: isPerson.Organization,
			Department:   isPerson.Department,
			Position:     isPerson.Position,
		}
		return &res, false, nil
	}
}

// TempImage получает изображение из файловой БД по его полному поути
func (m Db) TempImage(filePath string) ([]byte, error) {
	// Вычлиняем подпапку с часом
	//2020.12.13_13.27.28_525935.jpeg
	match := regexp.MustCompile(`^(\d+\.\d+\.\d+_\d+\.\d+\.\d+)_`).FindStringSubmatch(filePath)
	if len(match) == 0 {
		m.log.Warnf("некорректное имя файла изображения: %s", filePath)
		return nil, errors.Errorf("некорректное имя файла изображения: %s", filePath)
	}
	t, err := time.Parse("2006.01.02_15.04.05", match[1])
	if err != nil {
		m.log.Warnf("время в имени файла указано некорректно: %s", match[1])
		return nil, errors.Errorf("время в имени файла указано некорректно: %s", match[1])
	}
	fileName := filepath.Join(m.RootTemperatureDir, t.Format("2006.01.02"), fmt.Sprintf("%02d", t.Hour()), filePath)
	if _, err := os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			m.log.Warnf("не найден указанный файл: %s", fileName)
			return nil, errors.Annotatef(err, "не найден файл %s", fileName)
		}
		m.log.Errorf("ошибка чтения файла: %s", fileName)
		return nil, errors.Annotatef(err, "ошибка чтения %s", fileName)
	}
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return content, nil
}

// SetTempImage сохраняет изображение в файловую базу данных. Возвращает полный путь к сохранённому файлу
func (m Db) SetTempImage(create time.Time, wigand model.Wigand, content []byte) (*string, error) {
	fRelPath := filepath.Join(create.Format("2006.01.02"), create.Format("15"))
	fPath := filepath.Join(m.RootTemperatureDir, fRelPath)

	fName := fmt.Sprintf("%s_%d.jpeg", create.Format("2006.01.02_15.04.05"), wigand.ID)
	// Создаём директорию, если её нет
	if _, err := os.Stat(fPath); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(fPath, os.ModePerm); err != nil {
				m.log.Errorf("ошибка создания отсутсвующей директории %s: %s", fPath, err)
				return nil, errors.Trace(err)
			}
		} else {
			m.log.Errorf("ошибка создания директории %s: %s", fPath, err)
			return nil, errors.Trace(err)
		}
	}
	// Сохраняем файл, если его ещё нет
	if _, err := os.Stat(filepath.Join(fPath, fName)); err != nil && os.IsNotExist(err) {
		if err := ioutil.WriteFile(filepath.Join(fPath, fName), content, os.ModePerm); err != nil {
			m.log.Errorf("ошибка сохранения файла %s: %s", filepath.Join(fPath, fName), err)
			return nil, errors.Trace(err)
		}
	}
	return &fName, nil
}

// TemperatureLogByPerson получение лога температур для указаной персоны, за период, не более указанного
func (m Db) TemperatureLogByPerson(personID uint, duration time.Duration) ([]store.TemperatureLog, error) {
	timeLimit := time.Now().Add(-(duration * time.Second))
	temps := make([]Temperature, 0)
	if err := m.db.Where("person_id = ? AND created_at > ?", personID, timeLimit).Find(&temps).Error; err != nil {
		m.log.Error(err)
		return nil, errors.Trace(err)
	}
	// Определяем персону для каждого замера температуры
	result := make([]store.TemperatureLog, 0)
	for _, v := range temps {
		var person Person
		if err := m.db.Where("wigand = ?", v.PersonID).Take(&person).Error; err != nil {
			if gorm.ErrRecordNotFound.Error() == err.Error() {
				m.log.Warnf("для персоны %d с последней записи термопада %d не найдено описания (пропускаем запись)", v.PersonID, v.TermopadID)
				continue
			}
			return nil, errors.Trace(err)
		}
		// Формирование резлультата
		result = append(result, store.TemperatureLog{
			ID:         v.ID,
			TermopadID: v.TermopadID,
			CreatedAt:  &v.CreatedAt,
			Person: model.Person{
				Wigand:       model.Wigand{ID: uint(person.Wigand)},
				Family:       person.Family,
				Name:         person.Name,
				MiddleName:   person.MiddleName,
				Organization: person.Organization,
				Department:   person.Department,
				Position:     person.Position,
			},
		})
	}
	return result, nil
}

// TemperatureLogByTermopad получение лога температур по выбранному термопаду, за период, не более указанного
func (m Db) TemperatureLogByTermopad(termopadID uint, duration time.Duration) ([]store.TemperatureLog, error) {
	timeLimit := time.Now().Add(-(duration * time.Second))
	// Определяем основной лог температуры
	temps := make([]Temperature, 0)
	if err := m.db.Where("termopad_id = ? AND created_at > ?", termopadID, timeLimit).Find(&temps).Error; err != nil {
		m.log.Error(err)
		return nil, errors.Trace(err)
	}
	// Определяем персону для каждого замера температуры
	result := make([]store.TemperatureLog, 0)
	for _, v := range temps {
		var person Person
		if err := m.db.Where("wigand = ?", v.PersonID).Take(&person).Error; err != nil {
			if gorm.ErrRecordNotFound.Error() == err.Error() {
				m.log.Warnf("для персоны %d с последней записи термопада %d не найдено описания (пропускаем запись)", v.PersonID, termopadID)
				continue
			}
			return nil, errors.Trace(err)
		}
		// Создаём результат
		result = append(result, store.TemperatureLog{
			ID:         v.ID,
			TermopadID: v.TermopadID,
			CreatedAt:  &v.CreatedAt,
			Person: model.Person{
				Wigand:       model.Wigand{ID: uint(person.Wigand)},
				Family:       person.Family,
				Name:         person.Name,
				MiddleName:   person.MiddleName,
				Organization: person.Organization,
				Department:   person.Department,
				Position:     person.Position,
			},
		})
	}
	return result, nil
}

// SetTemperatureLog сохраняет основные данные о температуре и термопаде с именем imageName в лог базы данных
func (m Db) SetTemperatureLog(termopadID uint, wigandID uint, temperature float64, imageName string) error {
	tempLog := Temperature{
		PersonID:    int(wigandID),
		TermopadID:  int(termopadID),
		Temperature: math.Round(temperature*10) / 10,
		ImageName:   imageName,
	}
	if err := m.db.Create(&tempLog).Error; err != nil {
		m.log.Error(err)
		return errors.Trace(err)
	}
	return nil
}

// LastPerson возвращает описание последней замерившейся персоны и её температуры на термопаде.
// Если запись не найдена или не найдена персона для этой записи, возвращается ошибка, проверяемая Db.IsNotFound
func (m Db) LastPerson(termopadID uint) (*store.LastPerson, error) {
	var temperature Temperature
	var person Person

	if err := m.db.Where("termopad_id = ?", termopadID).Last(&temperature).Error; err != nil {
		if gorm.ErrRecordNotFound.Error() == err.Error() {
			m.log.Infof("не найдено последней записи для термопада %d", termopadID)
			return nil, gorm.ErrRecordNotFound
		}
		return nil, errors.Trace(err)
	}
	if err := m.db.Where("wigand = ?", temperature.PersonID).Take(&person).Error; err != nil {
		if gorm.ErrRecordNotFound.Error() == err.Error() {
			m.log.Warnf("для персоны %d с последней записи термопада %d не найдено описания", temperature.PersonID, termopadID)
			return nil, gorm.ErrRecordNotFound
		}
		return nil, errors.Trace(err)
	}

	res2 := store.LastPerson{
		CreatedAt: &temperature.CreatedAt,
		Termperature: model.Temperature{
			TermopadID:  uint(temperature.TermopadID),
			Wigand:      model.NewWigand(temperature.PersonID),
			Temperature: temperature.Temperature,
			ImagePath:   temperature.ImageName,
			Image:       nil,
		},
		Person: model.Person{
			CreateAt:     &person.CreatedAt,
			UpdateAt:     &person.UpdatedAt,
			Wigand:       model.Wigand{ID: uint(person.Wigand)},
			Family:       person.Family,
			Name:         person.Name,
			MiddleName:   person.MiddleName,
			Organization: person.Organization,
			Department:   person.Department,
			Position:     person.Position,
		},
	}
	return &res2, nil
}

// PersonLog возвращает значения температур для персоны с wigandID за days дней (со смещением offsetDays) по
// каждому замеру температуры. Если compact=true - данные замеров сжимаются до дней и температура показыватся
// только минимальная и максимальная для каждого дня
func (m Db) PersonLog(wigandID uint, days uint, offsetDays uint, compact bool) ([]model.TemperatureMetric, error) {
	// Получаем данные о персоне
	person, err := m.GetPerson(wigandID)
	if err != nil {
		if m.IsNotFound(err) {
			m.log.Warnf("персоны с wigandID=%d не найдена", wigandID)
			return nil, errors.Errorf("персоны с wigandID=%d не найдена", wigandID)
		}
		m.log.Warn(err)
		return nil, errors.Trace(err)
	}

	startDays, finishDays := m.calculateDate(days, offsetDays)
	rows := make([]Temperature, 0)
	if err := m.db.Where("person_id = ? AND created_at > ? AND created_at < ?", wigandID, startDays, finishDays).Find(&rows).Error; err != nil {
		if m.IsNotFound(err) {
			return make([]model.TemperatureMetric, 0), nil
		}
		m.log.Warn(err)
		return nil, errors.Trace(err)
	}

	// Подготовка резльтата (пока развёрнутного, для compact=false)
	result := make([]model.TemperatureMetric, 0)
	for _, v := range rows {

		// Получение инфромации о термопаде
		var termopad *model.TermopadInfo
		for _, t := range m.globalConfig.Termopad.Info {
			if t.ID == uint(v.TermopadID) {
				termopad = &model.TermopadInfo{
					ID:           uint(v.TermopadID),
					URL:          t.Address,
					SudosID:      t.Cabina,
					Name:         t.Name,
					SerialNumber: 0,
					Description:  t.Description,
				}
				break
			}
		}
		if termopad == nil {
			m.log.Warnf("термопад с ID:%v не описан в конфигурации", v.TermopadID)
			return nil, errors.Errorf("термопад с ID:%v не описан в конфигурации", v.TermopadID)
		}

		// Формирование результата
		result = append(result, model.TemperatureMetric{
			Date:           v.CreatedAt,
			Temperature:    math.Round(v.Temperature*10) / 10,
			TemperatureMax: 0,
			TemperatureMin: 0,
			Image:          v.ImageName,
			Person:         *person,
			Termopad:       *termopad,
		})
	}

	// Сжатие температуры при compact=true
	if compact {
		result = m.compactTemperature(result)
	}

	return result, nil
}

// TermopadLog возвращает значения температур для термопада с termopadID за days дней (со смещением offsetDays) по
// каждому замеру температуры. Если compact=true - данные замеров сжимаются до дней и температура показыватся
// только минимальная и максимальная для каждого дня
func (m Db) TermopadLog(termopadID uint, days uint, offsetDays uint, compact bool) ([]model.TemperatureMetric, error) {
	if termopadID == 0 {
		m.log.Warn("передан некорректный идентификатор термопада termopadID=0")
		return nil, errors.New("передан некорректный идентификатор термопада termopadID=0")
	}
	startDays, finishDays := m.calculateDate(days, offsetDays)
	rows := make([]Temperature, 0)
	if err := m.db.Where("termopad_id = ? AND created_at > ? AND created_at < ?", termopadID, startDays, finishDays).Find(&rows).Error; err != nil {
		if m.IsNotFound(err) {
			return make([]model.TemperatureMetric, 0), nil
		}
		m.log.Warn(err)
		return nil, errors.Trace(err)
	}

	// Подготовка резльтата (пока развёрнутного, для compact=false)
	result := make([]model.TemperatureMetric, 0)
	for _, v := range rows {

		// Получаем информацию о персоне из кэша или БД
		wigandKey := strconv.Itoa(v.PersonID)
		var personDb Person
		if p, found := m.personCache.Get(wigandKey); found {
			personDb = p.(Person)
		} else {
			if err := m.db.Where("wigand = ?", v.PersonID).Take(&personDb).Error; err != nil {
				if m.IsNotFound(err) {
					m.log.Warnf("персоны с ID:%v в БД не найдена", v.PersonID)
					return nil, errors.Errorf("персоны с ID:%v в БД не найдена", v.PersonID)
				}
				m.log.Warn(err)
				return nil, errors.Trace(err)
			}
			m.personCache.Set(wigandKey, personDb, cache.DefaultExpiration)
		}
		person := model.Person{
			CreateAt:     &personDb.CreatedAt,
			UpdateAt:     &personDb.UpdatedAt,
			Wigand:       model.NewWigand(v.PersonID),
			Family:       personDb.Family,
			Name:         personDb.Name,
			MiddleName:   personDb.MiddleName,
			Organization: personDb.Organization,
			Department:   personDb.Department,
			Position:     personDb.Position,
		}

		// Получение инфромации о термопаде
		var termopad *model.TermopadInfo
		for _, t := range m.globalConfig.Termopad.Info {
			if t.ID == termopadID {
				termopad = &model.TermopadInfo{
					ID:           termopadID,
					URL:          t.Address,
					SudosID:      t.Cabina,
					Name:         t.Name,
					SerialNumber: 0,
					Description:  t.Description,
				}
				break
			}
		}
		if termopad == nil {
			m.log.Warnf("термопад с ID:%v не описан в конфигурации", termopadID)
			return nil, errors.Errorf("термопад с ID:%v не описан в конфигурации", termopadID)
		}

		// Формирование результата
		result = append(result, model.TemperatureMetric{
			Date:           v.CreatedAt,
			Temperature:    math.Round(v.Temperature*10) / 10,
			TemperatureMax: 0,
			TemperatureMin: 0,
			Image:          v.ImageName,
			Person:         person,
			Termopad:       *termopad,
		})
	}

	// Сжатие температуры при compact=true
	if compact {
		result = m.compactTemperature(result)
	}

	return result, nil
}

// Сжатие лога температуры до однодневного лога с указанием максимальной и минимальной температуры
func (m Db) compactTemperature(temperature []model.TemperatureMetric) []model.TemperatureMetric {

	// Делаем промежуточную карту для объединения температур в один день
	cacheLoc := make(map[string]model.TemperatureMetric)
	for _, v := range temperature {
		// Обединяем температуры в один день
		date := tool.RoundToDate(v.Date)
		dateStr := date.Format("2006.01.02")
		if _, ok := cacheLoc[dateStr]; !ok {
			v.Date = date
			v.TemperatureMax = v.Temperature
			v.TemperatureMin = v.Temperature
			cacheLoc[dateStr] = v
		} else {
			c := cacheLoc[dateStr]
			if v.Temperature > cacheLoc[dateStr].TemperatureMax {
				c.TemperatureMax = v.Temperature
			} else if v.Temperature < cacheLoc[dateStr].TemperatureMin {
				c.TemperatureMin = v.Temperature
			}
			cacheLoc[dateStr] = c
		}
	}

	// Формируем окончательный результат
	result := make([]model.TemperatureMetric, 0)
	for _, v := range cacheLoc {
		v.Temperature = 0
		v.Person.Wigand.ID = 0
		v.Person.Name = ""
		v.Person.MiddleName = ""
		v.Person.Family = ""
		v.Person.Organization = ""
		v.Person.Department = ""
		v.Person.Position = ""
		result = append(result, v)
	}

	// Окончательная сортировка
	sort.Slice(result, func(i, j int) bool { return result[i].Date.String() < result[j].Date.String() })
	return result
}

// Вычисляет, начиная с текущей даты колличество дней days со смещением offset дней. Возвращается начало
// периода в startDate до finishDate
func (m Db) calculateDate(days uint, offset uint) (startDate, finishDate time.Time) {
	finishDate = time.Now().Add(-(time.Duration(offset) * time.Hour * 24))
	startDate = finishDate.Add(-(time.Duration(days) * time.Hour * 24)) // Всего дней
	return startDate, finishDate
}

// PersonImage возвращает путь до изображения персоны
func (m Db) PersonImage(wigand uint) ([]byte, error) {
	file := filepath.Join(m.RootPersonDir, fmt.Sprintf("%d.jpeg", wigand))
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return content, nil
}

// SetPersonImage сохраняет изображения персоны в БД и возвращает имя созданного файла
func (m Db) SetPersonImage(wigand uint, content []byte) error {
	fPath := m.RootPersonDir
	fName := fmt.Sprintf("%d.jpeg", wigand)
	// Создаём директорию, если её нет
	if _, err := os.Stat(fPath); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(fPath, os.ModePerm); err != nil {
				m.log.Errorf("ошибка создания отсутсвующей директории %s: %s", fPath, err)
				return errors.Trace(err)
			}
		} else {
			m.log.Errorf("ошибка создания директории %s: %s", fPath, err)
			return errors.Trace(err)
		}
	}
	// Сохраняем файл, если его ещё нет
	if _, err := os.Stat(filepath.Join(fPath, fName)); err != nil && os.IsNotExist(err) {
		if err := ioutil.WriteFile(filepath.Join(fPath, fName), content, os.ModePerm); err != nil {
			m.log.Errorf("ошибка сохранения файла %s: %s", filepath.Join(fPath, fName), err)
			return errors.Trace(err)
		}
	}
	return nil
}

// Clean очищает записи в БД старше времени, указанного в duration
// todo: в процессе создания
func (m Db) Clean(days int) error {
	// 1) удаляем все записи в БД, старше указанных дней
	// 2) удаляем все директории с изображениями замеров, старше указанных дней

	m.log.Info("запуск процесса очистки старых данных архива")

	// Удаление записей в базе данных

	lastDate, _ := m.calculateDate(uint(days), 0)
	rows := make([]Temperature, 0)
	if err := m.db.Where("created_at > ?", lastDate).Find(&rows).Error; err != nil {
		if m.IsNotFound(err) {
			m.log.Info("записей в архиве для удаления нет")
			return nil
		}
		m.log.Warn(err)
		return errors.Trace(err)
	}

	//pp.Println(len(rows))

	// Удаляем записи архивов

	dir, err := os.Open(m.RootTemperatureDir)
	if err != nil {
		m.log.Warn(err)
		return errors.Trace(err)
	}
	defer func() { _ = dir.Close() }()

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		m.log.Warn(err)
		return errors.Trace(err)
	}
	deleteDirs := make([]string, 0)
	for _, fi := range fileInfos {
		re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
		match := re.FindStringSubmatch(fi.Name())
		if len(match) > 0 {
			year, _ := strconv.Atoi(match[1])
			month, _ := strconv.Atoi(match[2])
			day, _ := strconv.Atoi(match[3])
			dirDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
			if lastDate.After(dirDate) {
				deleteDirs = append(deleteDirs, fi.Name())
			}
		}
	}

	//pp.Println(days, lastDate)
	//pp.Println(deleteDirs)

	return nil
}
