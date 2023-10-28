package middlewares

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	"github.com/HellfastUSMC/gophermart/internal/storage"
)

type CHRespWriter struct {
	http.ResponseWriter
}

func CheckAuth(log logger.CLogger, tokens map[string]storage.Token) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			credentials := req.Header.Get("Authorization")
			if strings.Contains(req.URL.String(), "register") || strings.Contains(req.URL.String(), "login") {
				h.ServeHTTP(res, req)
				return
			}
			if credentials != "" {
				for _, val := range tokens {
					if val.Token == credentials {
						h.ServeHTTP(res, req)
						return
					}
				}
				log.Error().Err(fmt.Errorf("auth error")).Msg(fmt.Sprintf("Somebody tried to open %s with none credentials", req.URL.String()))
				http.Error(res, "Credentials are missing", http.StatusUnauthorized)
				return
			} else {
				log.Error().Err(fmt.Errorf("auth error")).Msg(fmt.Sprintf("Somebody tried to open %s with none credentials", req.URL.String()))
				http.Error(res, "Credentials are missing", http.StatusUnauthorized)
				return
			}
		})
	}
}
