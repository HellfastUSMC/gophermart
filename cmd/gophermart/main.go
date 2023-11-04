package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/cashback_connector"
	"github.com/HellfastUSMC/gophermart/internal/config"
	"github.com/HellfastUSMC/gophermart/internal/controllers"
	"github.com/HellfastUSMC/gophermart/internal/database_connector"
	"github.com/HellfastUSMC/gophermart/internal/storage"
	"github.com/rs/zerolog"
)

func main() {
	log := zerolog.New(os.Stdout).Level(zerolog.TraceLevel).With().Timestamp().Logger()
	conf, err := config.GetStartupConfigData()
	if err != nil {
		log.Error().Err(err).Msg("config create error")
	}
	//currentStats := storage.NewCurrentStats()
	dbConn, err := dbconnector.NewConnectionPGSQL(conf.DBConnString, &log)
	if err != nil {
		log.Error().Err(err).Msg("DB connection error")
		return
	}
	store := storage.NewStorage(&log)
	cbConn := cbconnector.NewCBConnector(&log, conf.CashbackAddr)
	stat := storage.NewCurrentStats()
	controller := controllers.NewGmartController(&log, conf, store, dbConn, cbConn, stat)
	tickCheckTokens := time.NewTicker(1 * time.Hour)
	tickCheckCashback := time.NewTicker(1 * time.Second)
	tickCheckStats := time.NewTicker(1 * time.Hour)
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
			<-tickCheckStats.C
			controller.CheckStatus()
		}
	}()
	go func() {
		defer runtime.Goexit()
		for {
			<-tickCheckCashback.C
			orders, err := controller.PGConn.GetOrdersToCheck()
			if err != nil {
				log.Error().Err(err).Msg("error when get orders to update")
			}
			err = controller.Cashback.CheckOrders(orders, controller.PGConn.UpdateOrder, controller.PGConn.RegisterBonusChange)
			if err != nil {
				log.Error().Err(err).Msg("error when check orders")
			}

		}
	}()
	router := chi.NewRouter()
	router.Mount("/", controller.Route())
	log.Info().Msg(fmt.Sprintf(
		"Starting server at %s with check interval none, DB path %s and remote addr %s",
		controller.Config.GetServiceAddress(),
		controller.Config.GetDBPath(),
		controller.Config.GetCBPath(),
	))
	err = http.ListenAndServe(controller.Config.GetServiceAddress(), controller.Route())
	if err != nil {
		log.Error().Err(err)
	}
	select {}
}
