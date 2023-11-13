package storage

import (
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
)

type Storage struct {
	Logger logger.Logger
	Tokens map[string]Token
	Connector
}

type Connector interface {
	Close() error
	Ping() error
	UserOps
	OrderOps
	BonusOps
}

type UserOps interface {
	RegisterUser(login string, password string) (int64, error)
	CheckUserCreds(login string, plainPassword string) (bool, error)
	//GetUserBalance(login string) (float64, float64, error)
	CheckUserBalance(userLogin string) (float64, float64, error)
	UpdateUserBalance(checkUserBalance func(string) (float64, float64, error), userLogin string, sum float64, sub bool) (int64, error)
}

type OrderOps interface {
	GetUserOrders(login string) ([]Order, error)
	UpdateOrder(orderID string, accrual float64, status string) (int64, error)
	RegisterOrder(orderID string, accrual float64, placedAt string, login string) (int64, error)
	GetOrder(order string) (Order, error)
	GetOrdersToCheck() ([]Order, error)
}

type BonusOps interface {
	GetUserWithdrawals(login string) ([]Bonus, error)
	RegisterBonusChange(orderID string, sum float64, placedAt string, login string, sub bool) (int64, error)
}

type Token struct {
	Created time.Time
	User    string
	Token   string
}

func NewStorage(Logger logger.Logger, connector Connector) *Storage {
	return &Storage{
		Logger:    Logger,
		Tokens:    make(map[string]Token),
		Connector: connector,
	}
}

type UserCred struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Order struct {
	ID      string  `json:"number"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
	Date    string  `json:"uploaded_at"`
	Login   string  `json:"-"`
}

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Bonus struct {
	ID          int64   `json:"-"`
	OrderID     string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
	Login       string  `json:"-"`
}

type CurrentStats struct {
	DBConn       bool `json:"db_conn"`
	CashbackServ bool `json:"cashback_serv"`
}

func NewCurrentStats() *CurrentStats {
	return &CurrentStats{
		DBConn:       false,
		CashbackServ: false,
	}
}
