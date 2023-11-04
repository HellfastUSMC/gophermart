package cbConnector

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/HellfastUSMC/gophermart/internal/interfaces"
	"io"
	"net/http"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/storage"
)

type CBConnector struct {
	CBPath string
	Logger interfaces.Logger
}

func (c *CBConnector) CheckOrders(orders []storage.Order, updateOrderFunc func(id string, accrual float64, status string) (int64, error)) error {
	for _, val := range orders {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf("%s/api/orders/%s", c.CBPath, val.ID),
			nil,
		)
		if err != nil {
			return err
		}
		client := &http.Client{}
		response, err := client.Do(r)
		if err != nil {
			c.Logger.Error().Err(err).Msg("error in sending request request to CB")
			return err
		}
		rBody, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		order := storage.Order{}
		if response.StatusCode == http.StatusOK {
			err = json.Unmarshal(rBody, &order)
			if err != nil {
				c.Logger.Error().Err(err).Msg("error in unmarshal response body")
				return err
			}
			_, err = updateOrderFunc(val.ID, order.Accrual, order.Status)
			c.Logger.Info().Msg(fmt.Sprintf("order %s updated with status %s", order.ID, order.Status))
			if err != nil {
				c.Logger.Error().Err(err).Msg("cannot update order to DB")
				return err
			}
		} else if response.StatusCode == http.StatusNoContent {
			_, err = updateOrderFunc(val.ID, val.Accrual, "INVALID")
			if err != nil {
				c.Logger.Error().Err(err).Msg("cannot update order to DB")
				return err
			}
		} else {
			return fmt.Errorf("CB service returned status %d", response.StatusCode)
		}

		err = response.Body.Close()
		if err != nil {
			c.Logger.Error().Err(err).Msg("cannot close response body")
			return err
		}
	}
	return nil
}

func NewCBConnector(logger interfaces.Logger, CBPath string) *CBConnector {
	return &CBConnector{
		CBPath: CBPath,
		Logger: logger,
	}
}
