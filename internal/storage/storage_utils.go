package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

func CheckOrderStatus(orderID string, cashbackAddr string, log logger.CLogger, pg *PGSQLConn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("http://%s/api/orders/%s", cashbackAddr, orderID),
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msg("cannot make request to CB")
		return err
	}
	client := &http.Client{}
	response, err := client.Do(r)
	if err != nil {
		log.Error().Err(err).Msg("error in sending request request to CB")
		return err
	}
	rBody, err := io.ReadAll(response.Body)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error in rows")
		return err
	}
	fmt.Println(string(rBody), "second res body CB")
	order := Order{}
	err = json.Unmarshal(rBody, &order)
	if err != nil {
		log.Error().Err(err).Msg("error in unmarshal response body")
		return err
	}
	err = response.Body.Close()
	if err != nil {
		log.Error().Err(err).Msg("error in closing response body")
		return err
	}
	if order.Status == "INVALID" {
		log.Error().Msg("order rejected from CB")
		return err
	}
	if order.Status == "PROCESSED" {
		order.Date = time.Now().Format(time.RFC3339)
		_, err := pg.UpdateOrder(order.ID, order.Accrual, order.Status)
		if err != nil {
			log.Error().Msg("cannot register order to DB")
			return err
		}
	}
	return nil
}

func PasswordHasher(plainPass string) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plainPass), bcrypt.DefaultCost)
	return bytes, err
}

func CheckPasswordHash(password, hash []byte) (bool, error) {
	err := bcrypt.CompareHashAndPassword(hash, password)
	if err != nil {
		log.Error().Msg("error compare passwords")
		return false, err
	}
	return true, nil
}

func NewConnectionPGSQL(connPath string, logger logger.CLogger) (*PGSQLConn, error) {
	db, err := sql.Open("pgx", connPath)
	if err != nil {
		return nil, err
	}
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return nil, err
	}
	return &PGSQLConn{
		ConnectionString: connPath,
		DBConn:           db,
		Logger:           logger,
	}, nil
}
