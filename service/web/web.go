package web

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/kirsrus/termopad/server2/model"
	"github.com/kirsrus/termopad/server2/pkg/validator"
	"github.com/kirsrus/termopad/server2/service"
	"github.com/kirsrus/termopad/server2/service/web/graph"
	"github.com/kirsrus/termopad/server2/service/web/graph/generated"
	"github.com/kirsrus/termopad/server2/store"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/websocket"
	"github.com/juju/errors"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sirupsen/logrus"
)

const (
	waitRestartStartServer = 10 * time.Second
	webPort                = 80
	assetsDir              = "./assets/main"
	personPhotoDir         = "./imagedb"
)

// ConfigWeb конфигурация структуры Web
type ConfigWeb struct {
	Log *logrus.Logger

	WebPort        uint
	AssetsDir      string
	PersonPhotoDir string

	TermopadsOnPage uint
	MaxTemperature  float64
	MinTemperature  float64
}

// Web служба WEB-сервисов. Инициализируется через WebNew
type Web struct {
	ctx               context.Context
	log               *logrus.Entry
	validator         *validator.Validator
	e                 *echo.Echo
	graphqlHandler    *handler.Server
	playgroundHandler http.HandlerFunc
	resolver          *graph.Resolver

	dbStore store.DbStore

	webPort        uint
	assetsDir      string
	personPhotoDir string

	termopadsOnPage uint
	maxTemperature  float64
	minTemperature  float64
}

// NewWeb конструктор структкуры Web
func NewWeb(ctx context.Context, termopads []model.TermopadInfo, dbStore store.DbStore, config *ConfigWeb) (service.WebSvc, error) {
	var err error
	if config == nil {
		return nil, errors.New("не установлена конфигурация")
	}
	if config.Log == nil {
		config.Log = logrus.New()
		config.Log.Out = ioutil.Discard
	}
	web := Web{
		ctx: ctx,
		log: config.Log.WithFields(map[string]interface{}{
			"module": "web",
			"scope":  "service",
		}),
		validator: validator.Get(),
		e:         echo.New(),

		dbStore: dbStore,

		webPort:        webPort,
		assetsDir:      assetsDir,
		personPhotoDir: personPhotoDir,

		termopadsOnPage: 16,
		maxTemperature:  37.5,
		minTemperature:  35.0,
	}

	if config.WebPort != 0 {
		web.webPort = config.WebPort
	}
	if config.AssetsDir != "" {
		web.assetsDir = config.AssetsDir
	}
	if config.PersonPhotoDir != "" {
		web.personPhotoDir = config.PersonPhotoDir
	}
	if config.TermopadsOnPage != 0 {
		web.termopadsOnPage = config.TermopadsOnPage
	}
	if config.MaxTemperature != 0 {
		web.maxTemperature = config.MaxTemperature
	}
	if config.MinTemperature != 0 {
		web.minTemperature = config.MinTemperature
	}

	// Настойка WEB-сервера с поддержкой GraphQL
	web.e.HideBanner = true
	web.e.HidePort = true
	//web.e.Use(middleware.Logger())
	web.e.Use(middleware.Recover())
	web.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))
	// Точки входа в GrahpQL
	web.resolver, err = graph.NewResolver(termopads, dbStore, &graph.ConfigResolver{
		Log:             config.Log,
		TermopadsOnPage: web.termopadsOnPage,
		MaxTemperature:  web.maxTemperature,
		MinTemperature:  web.minTemperature,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	web.graphqlHandler = handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: web.resolver}))
	web.graphqlHandler.Use(extension.Introspection{})
	web.graphqlHandler.AddTransport(transport.POST{})
	web.graphqlHandler.AddTransport(
		transport.Websocket{
			KeepAlivePingInterval: 10 * time.Second, // Каждые 10 секунд подавать в канал (ping), иначе клиент его закроет
			Upgrader: websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
			},
		})
	web.playgroundHandler = playground.Handler("GraphQL", "/api")

	go web.serve()

	return &web, nil
}

func (m Web) serve() {
	for {
		m.log.Infof("старт HTTP-сервера на порту :%d", m.webPort)
		err := m.e.Start(fmt.Sprintf(":%d", m.webPort))
		m.log.Errorf("сервер неожиданно завершил работу: %s", err.Error())
		time.Sleep(waitRestartStartServer)
	}
}

func (m Web) GraphQLApi(path string) {
	m.e.GET(path, func(c echo.Context) error {
		req := c.Request()
		res := c.Response()
		m.graphqlHandler.ServeHTTP(res, req)
		return nil
	})
	m.e.POST(path, func(c echo.Context) error {
		req := c.Request()
		res := c.Response()
		m.graphqlHandler.ServeHTTP(res, req)
		return nil
	})
}

func (m Web) GraphQLPlayground(path string) {
	m.e.GET(path, func(c echo.Context) error {
		req := c.Request()
		res := c.Response()
		m.playgroundHandler.ServeHTTP(res, req)
		return nil
	})
}

func (m Web) TemperatureImage(path string) {
	m.e.GET(path, func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "не передано имя файла изображения"})
		}
		// Входящая строка: "2020.11.18_13.29.30_530619.jpeg"
		match := regexp.MustCompile(`(\d+\.\d+\.\d+)_(\d+)`).FindStringSubmatch(name)
		if len(match) == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "некорретный формат имени файла"})
		}
		content, err := m.dbStore.TempImage(name)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "ошибка: " + err.Error()})
		}
		mime := mimetype.Detect(content).String()
		return c.Blob(http.StatusOK, mime, content)
	})
}

func (m Web) PersonImage(path string) {
	m.e.GET(path, func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "не передан идентификатор персоны"})
		}
		wigand, err := strconv.Atoi(name)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": fmt.Sprintf("некорректный идентификатор персоны: %s", name)})
		}
		content, err := m.dbStore.PersonImage(uint(wigand))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "ошибка: " + err.Error()})
		}
		mime := mimetype.Detect(content).String()
		return c.Blob(http.StatusOK, mime, content)
	})
}

// Static ожидаем имя изображения в параметре name
func (m Web) Static(path string) {
	m.e.Static(path, m.assetsDir)
}

// TemperatureChanged температура изменена
func (m Web) TemperatureChanged(temperature model.TemperatureChange) {
	m.log.Debugf("отсылка температуры на WEB")
	m.resolver.TemperatureChanged(temperature)
}
