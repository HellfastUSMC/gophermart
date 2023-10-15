package controllers

import (
	"github.com/HellfastUSMC/gophermart/gophermart/internal/config"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/logger"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/middlewares"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/storage"
	"github.com/go-chi/chi/v5"
)

type gmartController struct {
	Logger       logger.CLogger
	Config       *config.SysConfig
	CurrentStats *storage.CurrentStats
	//MemStore serverstorage.MemStorekeeper
}

func (c *gmartController) Route() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middlewares.CheckAuth(c.Logger))
	router.Route("/", func(router chi.Router) {
		//router.Get("/", c.getAllStats)
		//router.Get("/ping", c.pingDB)
		//router.Post("/value/", c.returnJSONMetric)
		//router.Post("/update/", c.getJSONMetrics)
		//router.Post("/updates/", c.getJSONMetricsBatch)
		//router.Get("/value/{metricType}/{metricName}", c.returnMetric)
		//router.Post("/update/{metricType}/{metricName}/{metricValue}", c.getMetrics)
	})
	return router
}
