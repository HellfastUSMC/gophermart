package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

func CheckOrderStatus(orderID int64, cashbackAddr string, login string, log logger.CLogger, pg *PGSQLConn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("http://%s/api/orders/%d", cashbackAddr, orderID),
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
	} else if order.Status == "PROCESSED" {
		order.Date = time.Now()
		_, err := pg.RegisterOrder(order.ID, order.Accrual, order.Date.Format(time.RFC3339), login)
		if err != nil {
			log.Error().Msg("cannot register order to DB")
			return err
		}
	}
	return nil
}

func BasicCredDecode(encodedCredentials string) (login, password string, err error) {
	b64creds, err := base64.StdEncoding.DecodeString(encodedCredentials)
	loginPass := strings.Split(string(b64creds), ":")
	fmt.Println(loginPass, b64creds)
	if err != nil {
		return "", "", err
	}
	login = loginPass[0]
	password = loginPass[1]
	regEx := regexp.MustCompile(`^[+][0-9]{11}$`)
	if regEx.MatchString(login) {
		return login, password, nil
	} else {
		if string(login[0]) == "8" {
			newLogin := "7" + login[1:]
			login = newLogin
		}
		login = "+" + login
		if regEx.MatchString(login) {
			return login, password, nil
		}
	}
	return "", "", fmt.Errorf("cannot use provided credentials")
}

func PasswordHasher(plainPass string) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plainPass), bcrypt.DefaultCost)
	return bytes, err
}

func CheckPasswordHash(password, hash []byte) bool {
	err := bcrypt.CompareHashAndPassword(hash, password)
	return err == nil
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
