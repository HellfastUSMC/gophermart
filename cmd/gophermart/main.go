package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/cashback_connector"
	"github.com/HellfastUSMC/gophermart/internal/config"
	"github.com/HellfastUSMC/gophermart/internal/controllers"
	"github.com/HellfastUSMC/gophermart/internal/database_connector"
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
	conn, err := dbconnector.NewConnectionSQL(conf.DBConnString, &log)
	if err != nil {
		log.Error().Err(err).Msg("DB connection error")
		return
	}
	store := storage.NewStorage(&log, conn)
	cbConn := cbconnector.NewCBConnector(&log, conf.CashbackAddr)
	stat := storage.NewCurrentStats()
	controller := controllers.NewGmartController(&log, conf, store, cbConn, stat)
	tickCheckTokens := time.NewTicker(time.Duration(conf.TokensInterval) * time.Hour)
	tickCheckCashback := time.NewTicker(time.Duration(conf.OrdersInterval) * time.Second)
	tickCheckStats := time.NewTicker(time.Duration(conf.HealthInterval) * time.Hour)
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
			orders, err := controller.Storage.GetOrdersToCheck()
			if err != nil {
				log.Error().Err(err).Msg("error when get orders to update")
			}
			err = controller.Cashback.CheckOrders(orders, controller.Storage.UpdateOrder, controller.Storage.RegisterBonusChange)
			if err != nil {
				log.Error().Err(err).Msg("error when check orders")
			}

		}
	}()
	router := chi.NewRouter()
	router.Mount("/", controller.Route())
	log.Info().Msg(fmt.Sprintf(
		"Starting server at %s, DB path %s and remote addr %s",
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
