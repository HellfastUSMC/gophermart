package interfaces

import "github.com/HellfastUSMC/gophermart/internal/storage"

type Cashback interface {
	CheckOrders(
		orders []storage.Order,
		updateOrderFunc func(id string, accrual float64, status string) (int64, error),
	) error
}
