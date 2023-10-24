package middlewares

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/HellfastUSMC/gophermart/gophermart/internal/controllers"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/logger"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/storage"
)

type CHRespWriter struct {
	http.ResponseWriter
}

func CheckAuth(log logger.CLogger, controller *controllers.GmartController) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			credentials := req.Header.Get("Authorization")
			if credentials != "" && strings.Contains(req.Header.Get("Authorization"), "basic") {
				login, password, err := storage.BasicCredDecode(credentials)
				if err != nil {
					log.Error().Err(err).Msg("cannot decode credentials")
					http.Error(res, "Cannot decode credentials", http.StatusInternalServerError)
					return
				}
				exists, err := controller.CheckAuth(login, password)
				if err != nil {
					log.Error().Err(err).Msg("cannot check credentials")
					http.Error(res, "Cannot check credentials", http.StatusInternalServerError)
					return
				}
				if exists {
					h.ServeHTTP(res, req)
				}
				log.Error().Err(fmt.Errorf("auth error")).Msg(fmt.Sprintf("Somebody tried to open %s with wrong credentials", req.URL.String()))
				http.Error(res, "Credentials are wrong", http.StatusUnauthorized)

			} else if strings.Contains(req.URL.String(), "register") {
				h.ServeHTTP(res, req)
			}

			log.Error().Err(fmt.Errorf("auth error")).Msg(fmt.Sprintf("Somebody tried to open %s with none credentials", req.URL.String()))
			http.Error(res, "Credentials are missing", http.StatusUnauthorized)
			return
		})
	}
}
