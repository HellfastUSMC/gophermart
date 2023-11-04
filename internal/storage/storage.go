package storage

import (
	"time"

	"github.com/HellfastUSMC/gophermart/internal/interfaces"
)

type Storage struct {
	//Status *CurrentStats
	Logger interfaces.Logger
	Tokens map[string]Token
	Orders map[int64]Order
}

type Token struct {
	Created time.Time
	User    string
	Token   string
}

func NewStorage(Logger interfaces.Logger) *Storage {
	return &Storage{
		//Status: Status,
		Logger: Logger,
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

//type User struct {
//	ID              int64   `json:"id"`
//	Login           int64   `json:"phone"`
//	Password        string  `json:"-"`
//	Cashback        float64 `json:"cashback"`
//	AllTimeCashback float64 `json:"all_time_cashback"`
//}

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

//type CurrentStats struct {
//	DBConn       bool `json:"db_conn"`
//	CashbackServ bool `json:"cashback_serv"`
//}

//func NewCurrentStats() *CurrentStats {
//	return &CurrentStats{
//		LastUserID:   0,
//		LastOrderID:  0,
//		LastItemID:   0,
//		DBConn:       false,
//		CashbackServ: false,
//	}
//}
