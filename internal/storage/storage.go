package storage

import (
	"time"
)

var Roles = map[string]string{"admin": "admin", "moderator": "moderator", "user": "user"}

type Item struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Price   float64 `json:"price"`
	InStock bool    `json:"in-stock"`
}

type Order struct {
	ID       int64          `json:"id"`
	DateTime time.Time      `json:"date-time"`
	Total    float64        `json:"total"`
	Cashback float64        `json:"cashback"`
	Items    map[Item]int64 `json:"items"`
}

type User struct {
	ID        int64   `json:"id"`
	Role      string  `json:"role"`
	Firstname string  `json:"firstname'"`
	Lastname  string  `json:"lastname"`
	Phone     int64   `json:"phone"`
	Email     string  `json:"email"`
	City      string  `json:"city"`
	Street    string  `json:"street"`
	HouseNum  string  `json:"houseNum"`
	Orders    []Order `json:"orders"`
	Cashback  float64 `json:"cashback"`
	Password  string  `json:"password"`
}

type CurrentStats struct {
	LastOrderID  int64 `json:"last-order-id"`
	LastUserID   int64 `json:"last-user-id"`
	LastItemID   int64 `json:"last-item-id"`
	DBConn       bool  `json:"db-conn"`
	CashbackServ bool  `json:"cashback-serv"`
}

func NewCurrentStats() *CurrentStats {
	return &CurrentStats{
		LastUserID:   GetLastUserID(),
		LastOrderID:  GetLastOrderID(),
		LastItemID:   GetLastItemID(),
		DBConn:       CheckDB(),
		CashbackServ: CheckCashback(),
	}
}
