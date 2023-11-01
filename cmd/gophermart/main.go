package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/config"
	"github.com/HellfastUSMC/gophermart/internal/controllers"
	"github.com/HellfastUSMC/gophermart/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stdout).Level(zerolog.TraceLevel).With().Timestamp().Logger()
	conf, err := config.GetStartupConfigData()
	if err != nil {
		log.Error().Err(err).Msg("config create error")
	}
	currentStats := storage.NewCurrentStats()
	dbConn, err := storage.NewConnectionPGSQL(conf.DBConnString, &log)
	if err != nil {
		log.Error().Err(err).Msg("DB connection error")
		return
	}
	store := storage.NewStorage(dbConn, currentStats, &log)
	controller := controllers.NewGmartController(&log, conf, store)
	tickCheckTokens := time.NewTicker(1 * time.Hour)
	tickCheckCashback := time.NewTicker(1 * time.Second)
	go func() {
		defer runtime.Goexit()
		for {
			<-tickCheckTokens.C
			controller.CheckTokens()
		}
	}()
	go func() {
		defer runtime.Goexit()
		for {
			<-tickCheckCashback.C
			err := controller.CheckOrders()
			if err != nil {
				log.Error().Err(err).Msg("error when update orders")
			}
		}
	}()
	router := chi.NewRouter()
	router.Mount("/", controller.Route())
	log.Info().Msg(fmt.Sprintf(
		"Starting server at %s with check interval none, DB path %s and remote addr %s",
		controller.Config.GmartAddr,
		controller.Config.DBConnString,
		controller.Config.CashbackAddr,
	))
	err = http.ListenAndServe(controller.Config.GmartAddr, controller.Route())
	if err != nil {
		log.Error().Err(err)
	}
	select {}
}
