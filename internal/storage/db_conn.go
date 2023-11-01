package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type PGSQLConn struct {
	ConnectionString string
	DBConn           *sql.DB
	Logger           logger.CLogger
}

func (pg *PGSQLConn) Close() error {
	err := pg.DBConn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (pg *PGSQLConn) makeQueryContext(query string, args ...any) (*sql.Rows, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	f := func() (*sql.Rows, error) {
		rows, err := pg.DBConn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		return rows, nil
	}
	var netErr net.Error
	rows, err := retryReadFunc(2, 3, f, &netErr)
	if err != nil {
		return nil, cancel, err
	}
	return rows, cancel, nil
}

func (pg *PGSQLConn) makeExecContext(query string, args ...any) (int64, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	res, err := pg.DBConn.ExecContext(ctx, query, args...)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query from DB")
		return 0, cancel, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when get rows affected")
		return 0, cancel, err
	}
	return rows, cancel, nil
}

func (pg *PGSQLConn) GetUserBalance(login string) (float64, float64, error) {
	row, cancel := pg.makeQueryRowCTX("SELECT cashback, withdrawn FROM USERS WHERE login=$1", login)
	defer cancel()
	var balance float64
	var withdrawn float64
	err := row.Scan(&balance, &withdrawn)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when scanning rows")
		return 0, 0, err
	}
	return balance, withdrawn, nil
}

func (pg *PGSQLConn) GetUserWithdrawals(login string) ([]Withdraw, error) {
	rows, cancel, err := pg.makeQueryContext("SELECT * FROM WITHDRAWALS WHERE login=$1 ORDER BY placed_at", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query user withdraws from DB")
		return nil, err
	}
	var (
		withdrawals []Withdraw
		withdraw    Withdraw
	)
	for rows.Next() {
		err := rows.Scan(&withdraw.ID, &withdraw.OrderID, &withdraw.Sum, &withdraw.ProcessedAt, &withdraw.Login)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error when scanning rows")
			return nil, err
		}
		withdrawals = append(withdrawals, withdraw)
	}
	if rows.Err() != nil {
		pg.Logger.Error().Err(err).Msg("error in rows")
		return nil, err
	}
	err = rows.Close()
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when closing rows")
		return nil, err
	}
	if len(withdrawals) == 0 {
		return nil, nil
	}
	defer cancel()
	return withdrawals, nil
}

func (pg *PGSQLConn) makeQueryRowCTX(query string, args ...any) (*sql.Row, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	row := pg.DBConn.QueryRowContext(ctx, query, args...)
	return row, cancel
}

func (pg *PGSQLConn) GetOrder(order string) (Order, error) {
	row, cancel := pg.makeQueryRowCTX("SELECT * FROM ORDERS WHERE id=$1", order)
	defer cancel()
	var ord Order
	err := row.Scan(&ord.ID, &ord.Accrual, &ord.Date, &ord.Login, &ord.Status)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when scanning row")
		return ord, err
	}
	return ord, nil
}

func (pg *PGSQLConn) GetUserOrders(login string) ([]Order, error) {
	rows, cancel, err := pg.makeQueryContext("SELECT * FROM ORDERS WHERE login=$1 ORDER BY placed_at", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when searching user orders in DB")
		return nil, err
	}
	defer cancel()
	var (
		orders []Order
		order  Order
	)
	for rows.Next() {
		err := rows.Scan(&order.ID, &order.Accrual, &order.Date, &order.Login, &order.Status)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error in scanning rows")
			return nil, err
		}
		orders = append(orders, order)
	}
	err = rows.Err()
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error in rows")
		return nil, err
	}
	err = rows.Close()
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when closing rows")
		return nil, err
	}
	return orders, nil
}

func (pg *PGSQLConn) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	f := func() error {
		if err := pg.DBConn.PingContext(ctx); err != nil {
			return err
		}
		return nil
	}
	var netErr net.Error
	err := retryWriteFunc(2, 3, f, &netErr)
	if err != nil {
		return err
	}
	return nil
}

func retryReadFunc(
	interval int,
	attempts int,
	readFunc func() (*sql.Rows, error),
	errorToRetry *net.Error,
) (*sql.Rows, error) {
	if errorToRetry == nil {
		return nil, fmt.Errorf("please provide error to retry to")
	}
	if readFunc == nil {
		return nil, fmt.Errorf("no read func provided")
	}
	rows, err := readFunc()
	if err != nil {
		if errors.As(err, errorToRetry) {
			for i := 0; i < attempts; i++ {
				time.Sleep(time.Second * time.Duration(interval))
				rows, err = readFunc()
				if err == nil {
					return rows, nil
				}
			}
		}
		return nil, err
	}
	return rows, nil
}

func retryWriteFunc(
	interval int,
	attempts int,
	writeFunc func() error,
	errorToRetry *net.Error,
) error {
	if errorToRetry == nil {
		return fmt.Errorf("please provide error to retry to")
	}
	if writeFunc == nil {
		return fmt.Errorf("no write func provided")
	}
	err := writeFunc()
	if err != nil {
		if errors.As(err, errorToRetry) {
			for i := 0; i < attempts; i++ {
				time.Sleep(time.Second * time.Duration(interval))
				err = writeFunc()
				if err == nil {
					return nil
				}
			}
		}
		return err
	}
	return nil
}

func (pg *PGSQLConn) CheckUserBalance(userLogin string) (float64, float64, error) {
	row, cancel := pg.makeQueryRowCTX("SELECT cashback, withdrawn FROM USERS WHERE login=$1", userLogin)
	defer cancel()
	bal := Balance{}
	err := row.Scan(&bal.Current, &bal.Withdrawn)
	if err != nil {
		return 0, 0, err
	}
	return bal.Current, bal.Withdrawn, nil
}

func (pg *PGSQLConn) SubUserBalance(userLogin string, sum float64) (int64, error) {
	current, _, err := pg.CheckUserBalance(userLogin)
	if err != nil {
		return 0, err
	}
	if current < sum {
		return 0, fmt.Errorf("can't substract from balance, current=%f < order=%f", current, sum)
	}
	rows, cancel, err := pg.makeExecContext("UPDATE USERS SET cashback=cashback-$1, withdrawn=withdrawn+$1 WHERE login=$2", sum, userLogin)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) AddUserBalance(userLogin string, sum float64) (int64, error) {
	rows, cancel, err := pg.makeExecContext("UPDATE USERS SET cashback=cashback+$1, cashback_all=cashback+$1 WHERE login=$2", sum, userLogin)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) RegisterOrder(orderID string, accrual float64, placedAt string, login string) (int64, error) {
	rows, cancel, err := pg.makeExecContext("INSERT INTO ORDERS (id,cashback,placed_at,login,status) VALUES ($1,$2,$3,$4,$5)", orderID, accrual, placedAt, login, "NEW")
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) UpdateOrder(orderID string, accrual float64, status string) (int64, error) {
	rows, cancel, err := pg.makeExecContext("UPDATE ORDERS SET cashback=$1, status=$3 WHERE id=$2", accrual, orderID, status)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) RegisterWithdraw(orderID string, sum float64, placedAt string, login string) (int64, error) {
	rows, cancel, err := pg.makeExecContext("INSERT INTO WITHDRAWALS (order_id, sum ,placed_at, login) VALUES ($1,$2,$3,$4)", orderID, sum, placedAt, login)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) RegisterUser(login string, password string) (int64, error) {
	hashedPass, err := PasswordHasher(password)
	if err != nil {
		return 0, err
	}
	rows, cancel, err := pg.makeExecContext("INSERT INTO USERS (login,password,cashback,cashback_all,withdrawn) VALUES ($1,$2,$3,$4,$5)", login, string(hashedPass), 0, 0, 0)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) CheckUserCreds(login string, plainPassword string) (bool, error) {
	row, cancel := pg.makeQueryRowCTX("SELECT password FROM USERS WHERE LOGIN=$1", login)
	defer cancel()
	var userHashedPwd string
	err := row.Scan(&userHashedPwd)
	if err != nil {
		return false, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(userHashedPwd), []byte(plainPassword))
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error compare passwords")
		return false, err
	}
	return true, nil
}
