package interfaces

import "github.com/HellfastUSMC/gophermart/internal/storage"

type DBConnector interface {
	Close() error
	GetUserBalance(login string) (float64, float64, error)
	GetUserWithdrawals(login string) ([]storage.Withdraw, error)
	GetOrder(order string) (storage.Order, error)
	GetUserOrders(login string) ([]storage.Order, error)
	Ping() error
	CheckUserBalance(userLogin string) (float64, float64, error)
	SubUserBalance(userLogin string, sum float64) (int64, error)
	AddUserBalance(userLogin string, sum float64) (int64, error)
	RegisterOrder(orderID string, accrual float64, placedAt string, login string) (int64, error)
	UpdateOrder(orderID string, accrual float64, status string) (int64, error)
	RegisterWithdraw(orderID string, sum float64, placedAt string, login string) (int64, error)
	RegisterUser(login string, password string) (int64, error)
	CheckUserCreds(login string, plainPassword string) (bool, error)
	GetOrdersToCheck() ([]storage.Order, error)
}
