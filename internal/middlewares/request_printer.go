package middlewares

import (
	"bytes"
	"fmt"
	"github.com/HellfastUSMC/gophermart/internal/logger"
	"io"
	"net/http"
)

//
//type CHRespWriter struct {
//	http.ResponseWriter
//}

func RequestPrinter(log logger.CLogger) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				log.Error().Err(err).Msg("")
			}
			req.ContentLength = int64(len(body))
			req.Body = io.NopCloser(bytes.NewBuffer(body))
			log.Info().Msg(fmt.Sprintf(
				"%s request to %s with body %s and auth string (if exists) %s",
				req.Method,
				req.URL.String(),
				string(body),
				req.Header.Get("Authorization")))
			h.ServeHTTP(res, req)
		})
	}
}
