package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/kirsrus/termopad/server2/controller/manager"
	termopadCtlMod "github.com/kirsrus/termopad/server2/controller/termopad"
	"github.com/kirsrus/termopad/server2/model"
	"github.com/kirsrus/termopad/server2/pkg/config"
	"github.com/kirsrus/termopad/server2/pkg/logger"
	"github.com/kirsrus/termopad/server2/service"
	sudosStoreMod "github.com/kirsrus/termopad/server2/service/sudos"
	termopadStoreMod "github.com/kirsrus/termopad/server2/service/termopad"
	webSvcMod "github.com/kirsrus/termopad/server2/service/web"
	dbStoreMod "github.com/kirsrus/termopad/server2/store/db"

	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
)

var (
	cfg *config.Config
	log *logrus.Logger
)

func init() {
	cfg = config.Get()
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = logrus.WarnLevel
	}
	log = logger.GetWithConfig(logger.Config{
		File:    cfg.Log.Filename,
		Level:   level,
		Console: cfg.Log.Console,
	})
}

func main() {

	err := run()
	if err != nil {
		fmt.Printf("ОШИБКА: в процессе работы произошла ошибка: %v\n", err)
		fmt.Printf("Для подробностей смотри лог: %s/%s\n", cfg.Log.Path, cfg.Log.Filename)
		log.Fatal(errors.ErrorStack(err))
	}
}

func run() error {
	// Отлавливаем сигнал завершения работы программы
	chanInterrupt := make(chan os.Signal, 1)
	signal.Notify(chanInterrupt, os.Interrupt)

	done := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel

	// region Настройка БД

	dbStore, err := dbStoreMod.NewDb(ctx, &dbStoreMod.ConfigDb{
		Log:          log,
		DbFile:       cfg.Db.Filename,
		GlobalConfig: cfg,
	})
	if err != nil {
		return errors.Trace(err)
	}

	// endregion
	// region Инициализация термопадов
	// Формирование списка опрашиваемых термопадов и запуск их мониторинга

	termopads := make([]service.TermopadSvc, 0)
	termopadsInfo := make([]model.TermopadInfo, 0)
	for _, i := range cfg.Termopad.Info {
		termopadInfo := model.TermopadInfo{
			ID:           i.ID,
			URL:          i.Address,
			SudosID:      i.Cabina,
			Name:         i.Name,
			SerialNumber: 0,
			Description:  i.Description,
		}

		termoStore, err := termopadStoreMod.NewWebsocket(ctx, &termopadStoreMod.ConfigWebsocket{
			Log:          log,
			TermopadInfo: termopadInfo,
		})
		if err != nil {
			return errors.Trace(err)
		}
		termopads = append(termopads, termoStore)
		termopadsInfo = append(termopadsInfo, termopadInfo)
	}

	termopadsAll, err := termopadCtlMod.NewTermopad(ctx, termopads, dbStore, &termopadCtlMod.ConfigTermopad{
		Log: log,
	})
	if err != nil {
		return errors.Trace(err)
	}

	// endregion
	// region Настройка СУДОС

	sudosStore, err := sudosStoreMod.NewSudos(ctx, &sudosStoreMod.ConfigSudos{
		Log:            log,
		SudosUrl:       cfg.Sudos.Address,
		MaxTemperature: cfg.Termopad.MaxTemperature,
		MinTemperature: cfg.Termopad.MinTemperature,
	})
	if err != nil {
		return errors.Trace(err)
	}

	// endregion
	// region Контроллер WEB

	webSvc, err := webSvcMod.NewWeb(ctx, termopadsInfo, dbStore, &webSvcMod.ConfigWeb{
		Log:            log,
		PersonPhotoDir: cfg.Images.Path,
	})
	if err != nil {
		return errors.Trace(err)
	}

	webSvc.Static("/")
	webSvc.GraphQLApi("/api")
	webSvc.GraphQLPlayground("/playground")
	webSvc.TemperatureImage("/image/:name")
	webSvc.PersonImage("/person/:name")

	// endregion
	// region Менеджер управления всеми

	managerCtl, err := manager.NewManager(ctx, &manager.ConfigManager{
		Log:               log,
		TermopadCtl:       termopadsAll,
		SudosSvc:          sudosStore,
		WebSvc:            webSvc,
		DbStore:           dbStore,
		CleanBasePeriod:   time.Hour * 24 * time.Duration(cfg.Db.ArchiveDays),
		CleanBaseInterval: time.Minute * time.Duration(cfg.Db.CleanArchiveInterval),
	})
	if err != nil {
		return errors.Trace(err)
	}

	go func() {
		err := managerCtl.Serve()
		if err != nil && err.Error() != context.Canceled.Error() {
			done <- errors.Trace(err)
		}
		done <- nil
	}()

	// endregion

	// Процесс завершения работы
	select {
	case err := <-done:
		return errors.Trace(err)
	case <-chanInterrupt:
		log.Info("получена по каналу interrupt команда на завершение работы программы")
		cancel()
		time.Sleep(time.Second)
		return nil
	}
}
