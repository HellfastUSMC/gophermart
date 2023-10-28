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
	//tickCheckStatus := time.NewTicker(time.Duration(conf.CheckInterval) * time.Second)
	tickCheckTokens := time.NewTicker(1 * time.Hour)
	tickCheckOrders := time.NewTicker(1 * time.Second)
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
			<-tickCheckOrders.C
			err := controller.CheckOrder()
			if err != nil {
				log.Error().Err(err).Msg("error in checking orders")
			}
		}
	}()
	//go func() {
	//	defer runtime.Goexit()
	//	for {
	//		<-tickCheckStatus.C
	//		err := controller.RenewStatus()
	//		if err != nil {
	//			log.Error().Err(err).Msg("error in status renew")
	//		}
	//		log.Info().Msg("status renewed")
	//	}
	//}()
	router := chi.NewRouter()
	router.Mount("/", controller.Route())
	log.Info().Msg(fmt.Sprintf(
		"Starting server at %s with check interval %ds, DB path %s and remote addr %s",
		controller.Config.GmartAddr,
		controller.Config.CheckInterval,
		controller.Config.DBConnString,
		controller.Config.CashbackAddr,
	))
	err = http.ListenAndServe(controller.Config.GmartAddr, controller.Route())
	if err != nil {
		log.Error().Err(err)
	}
	select {}
}
