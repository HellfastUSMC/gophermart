package cbconnector

import "github.com/HellfastUSMC/gophermart/internal/storage"

type Cashback interface {
	CheckOrders(
		orders []storage.Order,
		updateOrderFunc func(id string, accrual float64, status string) (int64, error),
		registerBonusChange func(orderID string, sum float64, placedAt string, login string, sub bool) (int64, error),
	) error
	CheckStatus() error
	CheckOrder(orderID string) (storage.Order, int, error)
}
