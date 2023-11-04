package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	cbConnector "github.com/HellfastUSMC/gophermart/internal/cashback_connector"
	dbConnector "github.com/HellfastUSMC/gophermart/internal/database_connector"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/interfaces"
	"github.com/HellfastUSMC/gophermart/internal/middlewares"
	"github.com/HellfastUSMC/gophermart/internal/storage"
	"github.com/ShiraazMoollatjie/goluhn"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type GmartController struct {
	Logger   interfaces.Logger
	Config   interfaces.Configurator
	Storage  *storage.Storage
	Cashback interfaces.Cashback
	PGConn   interfaces.DBConnector
}

func (c *GmartController) Route() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middlewares.CheckAuth(c.Logger, c.Storage.Tokens))
	router.Use(middlewares.RequestPrinter(c.Logger))
	router.Route("/api/user", func(router chi.Router) {
		router.Get("/orders", c.getUserOrders)
		router.Get("/balance", c.getUserBalance)
		router.Get("/withdrawals", c.getUserWithdrawals)
		router.Post("/register", c.registerUser)
		router.Post("/login", c.loginUser)
		router.Post("/orders", c.postOrder)
		router.Post("/balance/withdraw", c.withdrawFromBalance)
	})
	return router
}
func (c *GmartController) withdrawFromBalance(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	token := req.Header.Get("Authorization")
	if err != nil {
		log.Error().Err(err).Msg("cannot read request body")
		http.Error(res, "cannot read request body", http.StatusInternalServerError)
		return
	}
	withdraw := storage.Withdraw{}
	err = json.Unmarshal(body, &withdraw)
	if err != nil {
		log.Error().Err(err).Msg("cannot unmarshal request body")
		http.Error(res, "cannot unmarshal request body", http.StatusInternalServerError)
		return
	}
	withdraw.ProcessedAt = time.Now().Format(time.RFC3339)
	for key, val := range c.Storage.Tokens {
		if val.Token == token {
			withdraw.Login = key
			break
		}
	}
	_, err = c.PGConn.RegisterWithdraw(withdraw.OrderID, withdraw.Sum, withdraw.ProcessedAt, withdraw.Login)
	if err != nil {
		log.Error().Err(err).Msg("cannot register withdraw")
		http.Error(res, "cannot register withdraw", http.StatusInternalServerError)
		return
	}
	_, err = c.PGConn.SubUserBalance(withdraw.Login, withdraw.Sum)
	if err != nil {
		log.Error().Err(err).Msg("cannot sub user balance")
		http.Error(res, "cannot sub user balance", http.StatusInternalServerError)
		return
	}
}

func (c *GmartController) CheckTokens() {
	for key, val := range c.Storage.Tokens {
		if time.Since(val.Created) > 1*time.Hour {
			delete(c.Storage.Tokens, key)
		}
	}
}

func (c *GmartController) postOrder(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	token := req.Header.Get("Authorization")
	if err != nil {
		log.Error().Err(err).Msg("cannot read request body")
		http.Error(res, "cannot read request body", http.StatusInternalServerError)
		return
	}
	login := ""
	for key, val := range c.Storage.Tokens {
		if val.Token == token {
			login = key
			break
		}
	}
	orderID := string(body)
	err = goluhn.Validate(orderID)
	if err != nil {
		log.Error().Err(err).Msg("wrong order number")
		http.Error(res, "wrong order number", http.StatusUnprocessableEntity)
		return
	}
	_, err = c.PGConn.RegisterOrder(orderID, 0, time.Now().Format(time.RFC3339), login)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			order, err := c.PGConn.GetOrder(orderID)
			if err != nil {
				log.Error().Err(err).Msg("error when searching for order in DB")
				http.Error(res, "error when searching for order in DB", http.StatusInternalServerError)
				return
			}
			if order.Login == login {
				orderJSON, err := json.Marshal(order)
				if err != nil {
					log.Error().Err(err).Msg("error when marshaling order")
					http.Error(res, "error when marshaling order", http.StatusInternalServerError)
					return
				}
				res.Header().Add("Content-Type", "application/json")
				res.Header().Add("Date", time.Now().Format(http.TimeFormat))
				res.WriteHeader(http.StatusOK)
				_, err = res.Write(orderJSON)
				if err != nil {
					log.Error().Err(err).Msg("cannot write response")
					http.Error(res, "cannot write response", http.StatusInternalServerError)
					return
				}
			}
		}
		log.Error().Err(err).Msg("order already exist")
		http.Error(res, "order already exist", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/orders/%s", c.Config.GetCBPath(), string(body)),
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msg("cannot make request to CB")
		http.Error(res, "cannot make request to CB", http.StatusInternalServerError)
		return
	}
	client := &http.Client{}
	response, err := client.Do(r)
	if err != nil {
		log.Error().Err(err).Msg("error in sending request request to CB")
		http.Error(res, "error in sending request request to CB", http.StatusInternalServerError)
		return
	}
	rBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error().Err(err).Msg("error in rows")
		http.Error(res, "error in rows", http.StatusInternalServerError)
	}
	if response.StatusCode == http.StatusNoContent {
		log.Error().Err(err).Msg("order not registered in cashback service")
		http.Error(res, "order not registered in cashback service", http.StatusAccepted)
		return
	}
	order := storage.Order{}
	err = json.Unmarshal(rBody, &order)
	if err != nil {
		log.Error().Err(err).Msg("error in unmarshal response body")
		http.Error(res, "error in unmarshal response body", http.StatusInternalServerError)
		return
	}
	err = response.Body.Close()
	if err != nil {
		log.Error().Err(err).Msg("error in closing response body")
		http.Error(res, "error in closing response body", http.StatusInternalServerError)
		return
	}
	if order.Status == "PROCESSED" {
		_, err = c.PGConn.UpdateOrder(order.ID, order.Accrual, order.Status)
		if err != nil {
			log.Error().Msg("error in update order in DB")
			http.Error(res, "error in update order in DB", http.StatusInternalServerError)
			return
		}
		_, err = c.PGConn.AddUserBalance(login, order.Accrual)
		if err != nil {
			log.Error().Msg("error in update user balance in DB")
			http.Error(res, "error in update user balance in DB", http.StatusInternalServerError)
			return
		}
	}
	if order.Status == "INVALID" {
		log.Error().Msg("order rejected from CB")
		http.Error(res, "order rejected from CB", http.StatusConflict)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusAccepted)
}

func (c *GmartController) loginUser(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("cannot read request body")
		http.Error(res, "cannot read request body", http.StatusInternalServerError)
		return
	}
	userCreds := storage.UserCred{}
	err = json.Unmarshal(body, &userCreds)
	if err != nil {
		log.Error().Err(err).Msg("cannot unmarshal request body")
		http.Error(res, "cannot unmarshal request body", http.StatusInternalServerError)
		return
	}
	if userCreds.Login == "" || userCreds.Password == "" {
		log.Error().Msg("login or password missing in body")
		http.Error(res, "login or password missing in body", http.StatusInternalServerError)
		return
	}
	auth, err := c.PGConn.CheckUserCreds(userCreds.Login, userCreds.Password)
	if err != nil {
		log.Error().Err(err).Msg("cannot check provided credentials")
		http.Error(res, "cannot check provided credentials", http.StatusInternalServerError)
		return
	}
	if !auth {
		log.Error().Err(err).Msg("provided credentials are incorrect")
		http.Error(res, "provided credentials are incorrect", http.StatusInternalServerError)
		return
	}
	resp := storage.Token{
		Created: time.Now(),
		User:    userCreds.Login,
		Token:   c.generateToken(),
	}
	c.Storage.Tokens[userCreds.Login] = resp
	respJSON, err := json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msg("cannot marshal response")
		http.Error(res, "cannot marshal response", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Authorization", resp.Token)
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusOK)
	if _, err = res.Write(respJSON); err != nil {
		c.Logger.Error().Err(err).Msg("error in writing response")
		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
		return
	}

}

func (c *GmartController) registerUser(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("cannot read request body")
		http.Error(res, "cannot read request body", http.StatusInternalServerError)
		return
	}
	userCreds := storage.UserCred{
		Login:    "",
		Password: "",
	}
	err = json.Unmarshal(body, &userCreds)
	if err != nil {
		log.Error().Err(err).Msg("cannot unmarshal request body")
		http.Error(res, "cannot unmarshal request body", http.StatusInternalServerError)
		return
	}
	if userCreds.Login == "" || userCreds.Password == "" {
		log.Error().Msg("login or password missing in body")
		http.Error(res, "login or password missing in body", http.StatusInternalServerError)
		return
	}
	_, err = c.PGConn.RegisterUser(userCreds.Login, userCreds.Password)
	if err != nil {
		log.Error().Err(err).Msg("cannot add user to DB")
		http.Error(res, "cannot add user to DB", http.StatusInternalServerError)
		return
	}
	token := storage.Token{
		Created: time.Now(),
		User:    userCreds.Login,
		Token:   c.generateToken(),
	}
	c.Storage.Tokens[userCreds.Login] = token
	tokenJSON, err := json.Marshal(map[string]string{"Token": token.Token})
	if err != nil {
		log.Error().Err(err).Msg("user registered, but can't marshal token")
		http.Error(res, "user registered, but can't marshal token", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.Header().Add("Authorization", token.Token)
	res.WriteHeader(http.StatusOK)
	_, err = res.Write(tokenJSON)
	if err != nil {
		log.Error().Err(err).Msg("cannot write response")
		http.Error(res, "cannot write response", http.StatusInternalServerError)
		return
	}
}

func (c *GmartController) generateToken() (token string) {
	data := make([]byte, 16)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (c *GmartController) getUserWithdrawals(res http.ResponseWriter, req *http.Request) {
	token := req.Header.Get("Authorization")
	login := c.findLoginByToken(token)
	withdrawals, err := c.PGConn.GetUserWithdrawals(login)
	if err != nil {
		log.Error().Err(err).Msg("cannot get user withdrawals")
		http.Error(res, "cannot get user withdrawals", http.StatusInternalServerError)
		return
	}
	if withdrawals == nil {
		log.Error().Err(err).Msg("no withdrawals found for this user")
		http.Error(res, "no withdrawals found for this user", http.StatusNoContent)
		return
	}
	withdrawalsJSON, err := json.Marshal(withdrawals)
	if err != nil {
		log.Error().Err(err).Msg("cannot marshal withdrawals")
		http.Error(res, "cannot marshal withdrawals", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusOK)
	if _, err = res.Write(withdrawalsJSON); err != nil {
		c.Logger.Error().Err(err).Msg("error in writing response")
		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
		return
	}
}

func (c *GmartController) getUserBalance(res http.ResponseWriter, req *http.Request) {
	token := req.Header.Get("Authorization")
	login := c.findLoginByToken(token)
	balance, withdrawn, err := c.PGConn.GetUserBalance(login)
	if err != nil {
		log.Error().Err(err).Msg("cannot get user balance")
		http.Error(res, "cannot get user balance", http.StatusInternalServerError)
		return
	}
	bal := storage.Balance{
		Current:   balance,
		Withdrawn: withdrawn,
	}
	balJSON, err := json.Marshal(bal)
	if err != nil {
		log.Error().Err(err).Msg("cannot marshal balance")
		http.Error(res, "cannot marshal balance", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusOK)
	if _, err = res.Write(balJSON); err != nil {
		c.Logger.Error().Err(err).Msg("error in writing response")
		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
		return
	}
}

func (c *GmartController) findLoginByToken(token string) string {
	for _, val := range c.Storage.Tokens {
		if val.Token == token {
			return val.User
		}
	}
	return ""
}

//func (c *GmartController) CheckOrders() error {
//	orders, err := c.PGConn.GetOrdersToCheck()
//	if err != nil {
//		return err
//	}
//	for _, val := range orders {
//		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//		defer cancel()
//		r, err := http.NewRequestWithContext(
//			ctx,
//			http.MethodGet,
//			fmt.Sprintf("%s/api/orders/%s", c.Config.GetCBPath(), val.ID),
//			nil,
//		)
//		if err != nil {
//			return err
//		}
//		client := &http.Client{}
//		response, err := client.Do(r)
//		if err != nil {
//			c.Logger.Error().Err(err).Msg("error in sending request request to CB")
//			return err
//		}
//		rBody, err := io.ReadAll(response.Body)
//		if err != nil {
//			return err
//		}
//		order := storage.Order{}
//		if response.StatusCode == http.StatusOK {
//			err = json.Unmarshal(rBody, &order)
//			if err != nil {
//				c.Logger.Error().Err(err).Msg("error in unmarshal response body")
//				return err
//			}
//			_, err = c.PGConn.UpdateOrder(val.ID, order.Accrual, order.Status)
//			c.Logger.Info().Msg(fmt.Sprintf("order %s updated with status %s", order.ID, order.Status))
//			if err != nil {
//				c.Logger.Error().Err(err).Msg("cannot update order to DB")
//				return err
//			}
//		} else if response.StatusCode == http.StatusNoContent {
//			_, err = c.PGConn.UpdateOrder(val.ID, val.Accrual, "INVALID")
//			if err != nil {
//				c.Logger.Error().Err(err).Msg("cannot update order to DB")
//				return err
//			}
//		} else {
//			return fmt.Errorf("CB service returned status %d", response.StatusCode)
//		}
//
//		err = response.Body.Close()
//		if err != nil {
//			c.Logger.Error().Err(err).Msg("cannot close response body")
//			return err
//		}
//	}
//	return nil
//}

func (c *GmartController) getUserOrders(res http.ResponseWriter, req *http.Request) {
	token := req.Header.Get("Authorization")
	login := c.findLoginByToken(token)
	if login == "" {
		log.Error().Msg("cannot get user login")
		http.Error(res, "cannot get user login", http.StatusInternalServerError)
		return
	}
	orders, err := c.PGConn.GetUserOrders(login)
	if orders == nil {
		log.Error().Err(err).Msg("no orders found for this user")
		http.Error(res, "no orders found for this user", http.StatusNoContent)
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("cannot get user orders")
		http.Error(res, "cannot get user orders", http.StatusInternalServerError)
		return
	}
	ordersJSON, err := json.Marshal(orders)
	if err != nil {
		log.Error().Err(err).Msg("cannot marshal orders")
		http.Error(res, "cannot marshal orders", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusOK)
	_, err = res.Write(ordersJSON)
	if err != nil {
		log.Error().Err(err).Msg("cannot write orders to response")
		http.Error(res, "cannot write orders to response", http.StatusInternalServerError)
		return
	}
}

//func (c *GmartController) getStatus(res http.ResponseWriter, req *http.Request) {
//	status := c.Storage.Status
//	jsonStatus, err := json.Marshal(status)
//	if err != nil {
//		c.Logger.Error().Err(err).Msg("error in marshaling current status")
//		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
//		return
//	}
//	res.Header().Add("Content-Type", "application/json")
//	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
//	res.WriteHeader(http.StatusOK)
//	_, err = res.Write(jsonStatus)
//	if err != nil {
//		c.Logger.Error().Err(err).Msg("error in writing current status to response")
//		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
//		return
//	}
//}

//func (c *GmartController) PingDB() {
//	if err := c.PGConn.Ping(); err != nil {
//		c.Storage.Status.DBConn = false
//	}
//	c.Storage.Status.DBConn = true
//}
//
//func (c *GmartController) PingCB() {
//	if err := c.PGConn.Ping(); err != nil {
//		c.Storage.Status.DBConn = false
//	}
//	c.Storage.Status.DBConn = true
//}

func (c *GmartController) CheckAuth(login string, password string) (bool, error) {
	exists, err := c.PGConn.CheckUserCreds(login, password)
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in check user credentials")
		return false, err
	}
	return exists, nil
}

func NewGmartController(logger interfaces.Logger, conf interfaces.Configurator, storage *storage.Storage, pgConn *dbConnector.PGSQLConn, cbConnector *cbConnector.CBConnector) *GmartController {
	return &GmartController{
		Logger:   logger,
		Config:   conf,
		Storage:  storage,
		PGConn:   pgConn,
		Cashback: cbConnector,
	}
}
