package storage

import (
	"time"

	"github.com/HellfastUSMC/gophermart/gophermart/internal/logger"
)

//var Roles = map[string]string{"admin": "admin", "moderator": "moderator", "user": "user"}

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
	ID      int64     `json:"number"`
	Status  string    `json:"status"`
	Accrual float64   `json:"accrual.exe"`
	Date    time.Time `json:"placed_at"`
	Login   string    `json:"-"`
}

//type Item struct {
//	ID      int64   `json:"id"`
//	Name    string  `json:"name"`
//	Price   float64 `json:"price"`
//	InStock bool    `json:"in_stock"`
//}

//type Order struct {
//	ID       int64   `json:"id"`
//	DateTime string  `json:"date_time"`
//	//Total    float64 `json:"total"`
//	Cashback float64 `json:"cashback"`
//	//Items    map[Item]int64 `json:"items"`
//}

type User struct {
	ID              int64   `json:"id"`
	Role            string  `json:"role"`
	Firstname       string  `json:"firstname'"`
	Lastname        string  `json:"lastname"`
	Login           int64   `json:"phone"`
	Email           string  `json:"email"`
	City            string  `json:"city"`
	Street          string  `json:"street"`
	HouseNum        string  `json:"houseNum"`
	Orders          []Order `json:"orders"`
	Cashback        float64 `json:"cashback"`
	Password        string  `json:"-"`
	TempToken       []byte  `json:"-"`
	AllTimeCashback float64 `json:"all_time_cashback"`
}

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdraw struct {
	ID          int64   `json:"id"`
	OrderID     int64   `json:"order"`
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

//func (c *CurrentStats) CheckConns() {
//	c.DBConn = CheckDB()
//	c.CashbackServ = CheckCashback()
//}

//func (c *CurrentStats) SetLastUserID(userID int64) {
//	c.LastUserID = userID
//}
//
//func (c *CurrentStats) SetLastItemID(itemID int64) {
//	c.LastItemID = itemID
//}
//
//func (c *CurrentStats) SetLastOrderID(orderID int64) {
//	c.LastOrderID = orderID
//}
//
//func (c *CurrentStats) SetDBStatus(DBStatus bool) {
//	c.DBConn = DBStatus
//}
//
//func (c *CurrentStats) SetCBStatus(CBStatus bool) {
//	c.CashbackServ = CBStatus
//}
//
//func (c *CurrentStats) GetLastUserID() (userID int64) {
//	return c.LastUserID
//}
//
//func (c *CurrentStats) GetLastItemID() (itemID int64) {
//	return c.LastItemID
//}
//
//func (c *CurrentStats) GetLastOrderID() (orderID int64) {
//	return c.LastOrderID
//}
//
//func (c *CurrentStats) GetDBStatus() (DBStatus bool) {
//	return c.DBConn
//}
//
//func (c *CurrentStats) GetCBStatus() (CBStatus bool) {
//	return c.CashbackServ
//}

func NewCurrentStats() *CurrentStats {
	return &CurrentStats{
		LastUserID:   0,
		LastOrderID:  0,
		LastItemID:   0,
		DBConn:       false,
		CashbackServ: false,
	}
}
