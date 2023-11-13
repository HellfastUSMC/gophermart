package cbconnector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	"github.com/HellfastUSMC/gophermart/internal/storage"
)

type CBConnector struct {
	CBPath string
	Logger logger.Logger
}

func (c *CBConnector) CheckStatus() error {
	conn, err := net.DialTimeout("tcp", c.CBPath, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func (c *CBConnector) CheckOrder(orderID string) (storage.Order, int, error) {
	var order storage.Order
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/orders/%s", c.CBPath, orderID),
		nil,
	)
	if err != nil {
		c.Logger.Error().Err(err).Msg("cannot make request to CB")
		return order, 0, err
	}
	client := &http.Client{}
	response, err := client.Do(r)
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in sending request request to CB")
		return order, 0, err
	}
	rBody, err := io.ReadAll(response.Body)
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in rows")
	}
	if response.StatusCode == http.StatusOK {
		err = json.Unmarshal(rBody, &order)
		if err != nil {
			c.Logger.Error().Err(err).Msg("error in unmarshal response body")
			return order, response.StatusCode, err
		}
	}
	err = response.Body.Close()
	if err != nil {
		c.Logger.Error().Err(err).Msg("error in closing response body")
		return order, response.StatusCode, err
	}
	return order, response.StatusCode, nil
}

func (c *CBConnector) CheckOrders(
	orders []storage.Order,
	updateOrderFunc func(id string, accrual float64, status string) (int64, error),
	registerBonusChange func(orderID string, sum float64, placedAt string, login string, sub bool) (int64, error),
) error {
	for _, val := range orders {
		order, statusCode, err := c.CheckOrder(val.ID)
		if err != nil {
			c.Logger.Error().Err(err).Msg("cannot check order in CB service")
			return err
		}
		if statusCode == http.StatusOK {
			_, err = updateOrderFunc(val.ID, order.Accrual, order.Status)
			c.Logger.Info().Msg(fmt.Sprintf("order %s updated with status %s", val.ID, order.Status))
			if err != nil {
				c.Logger.Error().Err(err).Msg("cannot update order to DB")
				return err
			}
			if order.Status == "PROCESSED" {
				_, err = registerBonusChange(val.ID, order.Accrual, val.Date, val.Login, false)
				if err != nil {
					c.Logger.Error().Err(err).Msg("cannot update bonuses to DB")
					return err
				}
			}
		} else if statusCode == http.StatusNoContent {
			_, err = updateOrderFunc(val.ID, val.Accrual, "INVALID")
			if err != nil {
				c.Logger.Error().Err(err).Msg("cannot update order to DB")
				return err
			}
		} else {
			return fmt.Errorf("CB service returned status %d", statusCode)
		}
	}
	return nil
}

func NewCBConnector(logger logger.Logger, CBPath string) *CBConnector {
	return &CBConnector{
		CBPath: CBPath,
		Logger: logger,
	}
}
