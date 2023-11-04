package storage

import (
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
)

type Storage struct {
	PGConn *PGSQLConn
	Status *CurrentStats
	Logger logger.CLogger
	Tokens map[string]Token
	Orders map[int64]Order
}

type Token struct {
	Created time.Time
	User    string
	Token   string
}

func NewStorage(PGConn *PGSQLConn, Status *CurrentStats, cLogger logger.CLogger) *Storage {
	return &Storage{
		PGConn: PGConn,
		Status: Status,
		Logger: cLogger,
		Tokens: make(map[string]Token),
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

type User struct {
	ID              int64   `json:"id"`
	Login           int64   `json:"phone"`
	Password        string  `json:"-"`
	Cashback        float64 `json:"cashback"`
	AllTimeCashback float64 `json:"all_time_cashback"`
}

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdraw struct {
	ID          int64   `json:"-"`
	OrderID     string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
	Login       string  `json:"-"`
}

type CurrentStats struct {
	LastOrderID  int64 `json:"last_order_id"`
	LastUserID   int64 `json:"last_user_id"`
	LastItemID   int64 `json:"last_item_id"`
	DBConn       bool  `json:"db_conn"`
	CashbackServ bool  `json:"cashback_serv"`
}

func NewCurrentStats() *CurrentStats {
	return &CurrentStats{
		LastUserID:   0,
		LastOrderID:  0,
		LastItemID:   0,
		DBConn:       false,
		CashbackServ: false,
	}
}
