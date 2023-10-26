package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/HellfastUSMC/gophermart/gophermart/internal/config"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/logger"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/middlewares"
	"github.com/HellfastUSMC/gophermart/gophermart/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type GmartController struct {
	Logger  logger.CLogger
	Config  *config.SysConfig
	Storage *storage.Storage
}

func (c *GmartController) Route() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middlewares.CheckAuth(c.Logger, c.Storage.PGConn))
	router.Route("/api/status", func(rout chi.Router) {
		router.Get("/", c.getStatus)
	})
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
	_, err = c.Storage.PGConn.RegisterWithdraw(withdraw.OrderID, withdraw.Sum, withdraw.ProcessedAt, withdraw.Login)
	if err != nil {
		log.Error().Err(err).Msg("cannot register withdraw")
		http.Error(res, "cannot register withdraw", http.StatusInternalServerError)
		return
	}
	_, err = c.Storage.PGConn.SubUserBalance(withdraw.Login, withdraw.Sum)
	if err != nil {
		log.Error().Err(err).Msg("cannot sub user balance")
		http.Error(res, "cannot sub user balance", http.StatusInternalServerError)
		return
	}
}

func (c *GmartController) CheckTokens() {
	for key, val := range c.Storage.Tokens {
		if time.Now().Sub(val.Created) > 1*time.Hour {
			delete(c.Storage.Tokens, key)
		}
	}
}

func (c *GmartController) CheckOrder() error {
	for _, val := range c.Storage.Orders {
		err := storage.CheckOrderStatus(val.ID, c.Config.CashbackAddr, val.Login, c.Logger, c.Storage.PGConn)
		if err != nil {
			c.Logger.Error().Err(err).Msg("error in checking order status")
			return err
		}
	}
	return nil
}

func (c *GmartController) postOrder(res http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	token := req.Header.Get("Authorization")
	if err != nil {
		log.Error().Err(err).Msg("cannot read request body")
		http.Error(res, "cannot read request body", http.StatusInternalServerError)
		return
	}
	orderID, err := strconv.ParseInt(string(body), 10, 64)
	if err != nil {
		log.Error().Err(err).Msg("cannot parse int64 from body")
		http.Error(res, "cannot parse int64 from body", http.StatusInternalServerError)
		return
	}
	exists, err := c.Storage.PGConn.CheckOrderIDExists(orderID)
	if err != nil {
		log.Error().Err(err).Msg("cannot check order ID in DB")
		http.Error(res, "cannot check order ID in DB", http.StatusInternalServerError)
		return
	}
	if exists {
		log.Error().Err(err).Msg("order ID already exists")
		http.Error(res, "order ID already exists", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/orders/%d", c.Config.CashbackAddr, orderID),
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
	if order.Status == "INVALID" {
		log.Error().Msg("order rejected from CB")
		http.Error(res, "order rejected from CB", http.StatusConflict)
		return
	}
	login := ""
	for key, val := range c.Storage.Tokens {
		if val.Token == token {
			login = key
			break
		}
	}
	c.Storage.Orders[orderID] = storage.Order{ID: orderID, Login: login}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusOK)
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
	auth, err := c.Storage.PGConn.CheckUserCreds(userCreds.Login, userCreds.Password)
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
	_, err = c.Storage.PGConn.RegisterUser(userCreds.Login, userCreds.Password)
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
	credentials := req.Header.Get("Authorization")
	login, _, err := storage.BasicCredDecode(credentials)
	if err != nil {
		log.Error().Err(err).Msg("cannot decode credentials")
		http.Error(res, "cannot decode credentials", http.StatusInternalServerError)
		return
	}
	withdrawals, err := c.Storage.PGConn.GetUserWithdrawals(login)
	if err != nil {
		log.Error().Err(err).Msg("cannot get user withdrawals")
		http.Error(res, "cannot get user withdrawals", http.StatusInternalServerError)
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
	credentials := req.Header.Get("Authorization")
	login, _, err := storage.BasicCredDecode(credentials)
	if err != nil {
		log.Error().Err(err).Msg("cannot decode credentials")
		http.Error(res, "cannot decode credentials", http.StatusInternalServerError)
		return
	}
	balance, allTimeBal, err := c.Storage.PGConn.GetUserBalance(login)
	if err != nil {
		log.Error().Err(err).Msg("cannot get user balance")
		http.Error(res, "cannot get user balance", http.StatusInternalServerError)
		return
	}
	bal := storage.Balance{
		Current:   balance,
		Withdrawn: allTimeBal,
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

func (c *GmartController) getUserOrders(res http.ResponseWriter, req *http.Request) {
	credentials := req.Header.Get("Authorization")
	login, _, err := storage.BasicCredDecode(credentials)
	if err != nil {
		log.Error().Err(err).Msg("cannot decode credentials")
		http.Error(res, "cannot decode credentials", http.StatusInternalServerError)
		return
	}
	userID, err := c.Storage.PGConn.GetUserID(login)
	if err != nil {
		log.Error().Err(err).Msg("cannot get user ID")
		http.Error(res, "cannot get user ID", http.StatusInternalServerError)
		return
	}
	orders, err := c.Storage.PGConn.GetUserOrders(userID)
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

func (c *GmartController) getStatus(res http.ResponseWriter, req *http.Request) {
	status := c.Storage.Status
	jsonStatus, err := json.Marshal(status)
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in marshaling current status")
		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.Header().Add("Date", time.Now().Format(http.TimeFormat))
	res.WriteHeader(http.StatusOK)
	_, err = res.Write(jsonStatus)
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in writing current status to response")
		http.Error(res, fmt.Sprintf("there's an error %e", err), http.StatusInternalServerError)
		return
	}
}

func (c *GmartController) RenewStatus() error {
	c.PingCB()
	c.PingDB()
	userID, err := c.Storage.PGConn.CheckLastID("USERS")
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in last user ID")
		return err
	}
	c.Storage.Status.LastUserID = userID
	orderID, err := c.Storage.PGConn.CheckLastID("ORDERS")
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in last order ID")
		return err
	}
	c.Storage.Status.LastOrderID = orderID
	itemID, err := c.Storage.PGConn.CheckLastID("ITEMS")
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in last item ID")
		return err
	}
	c.Storage.Status.LastItemID = itemID
	return nil
}

func (c *GmartController) PingDB() {
	if err := c.Storage.PGConn.Ping(); err != nil {
		c.Storage.Status.DBConn = false
	}
	c.Storage.Status.DBConn = true
}

func (c *GmartController) PingCB() {
	if err := c.Storage.PGConn.Ping(); err != nil {
		c.Storage.Status.DBConn = false
	}
	c.Storage.Status.DBConn = true
}

func (c *GmartController) CheckAuth(login string, password string) (bool, error) {
	exists, err := c.Storage.PGConn.CheckUserCreds(login, password)
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in check user credentials")
		return false, err
	}
	return exists, nil
}

func NewGmartController(logger logger.CLogger, conf *config.SysConfig, storage *storage.Storage) *GmartController {
	return &GmartController{
		Logger:  logger,
		Config:  conf,
		Storage: storage,
	}
}
