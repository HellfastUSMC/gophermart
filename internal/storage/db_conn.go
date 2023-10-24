package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/HellfastUSMC/gophermart/gophermart/internal/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
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

func (pg *PGSQLConn) makeQueryContext(query string, args ...any) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	f := func() (*sql.Rows, error) {
		rows, err := pg.DBConn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		return rows, nil
	}
	var netErr net.Error
	rows, err := retryReadFunc(2, 3, f, &netErr)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (pg *PGSQLConn) makeExecContext(query string, args ...any) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	res, err := pg.DBConn.ExecContext(ctx, query, args...)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query userID from DB")
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when get rows affected")
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) CheckOrderIDExists(orderID int64) (bool, error) {
	rows, err := pg.makeQueryContext("SELECT ID FROM ORDERS ORDER BY ID DESC WHERE login=$1 LIMIT 1", orderID)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query orderID from DB")
		return false, err
	}
	defer rows.Close()
	var orderIDDB int64
	for rows.Next() {
		err = rows.Scan(&orderIDDB)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error when scanning orderID from rows")
			return false, err
		}
	}
	if orderIDDB > -1 {
		return true, nil
	}
	return false, nil
}

func (pg *PGSQLConn) GetUserID(login string) (int64, error) {
	rows, err := pg.makeQueryContext("SELECT ID FROM USERS ORDER BY ID DESC WHERE login=$1", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query userID from DB")
		return 0, err
	}
	defer rows.Close()
	var userID int64
	for rows.Next() {
		err := rows.Scan(&userID)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error when scanning rows")
			return 0, err
		}
	}
	return userID, nil
}

func (pg *PGSQLConn) GetUserBalance(login string) (float64, float64, error) {
	rows, err := pg.makeQueryContext("SELECT cashback, cashback_all FROM USERS ORDER BY ID DESC WHERE login=$1", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query userID from DB")
		return 0, 0, err
	}
	defer rows.Close()
	var balance float64
	var allTimeBal float64
	for rows.Next() {
		err := rows.Scan(&balance, &allTimeBal)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error when scanning rows")
			return 0, 0, err
		}
	}
	return balance, allTimeBal, nil
}

func (pg *PGSQLConn) GetUserWithdrawals(login string) ([]Withdraw, error) {
	rows, err := pg.makeQueryContext("SELECT * FROM WITHDRAWALS ORDER BY ID DESC WHERE login=$1", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query userID from DB")
		return nil, err
	}
	defer rows.Close()
	var (
		withdrawals []Withdraw
		withdraw    Withdraw
	)
	for rows.Next() {
		err := rows.Scan(withdraw.ID, withdraw.OrderID, withdraw.Sum, withdraw.ProcessedAt)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error when scanning rows")
			return nil, err
		}
		withdrawals = append(withdrawals, withdraw)
	}
	return withdrawals, nil
}

func (pg *PGSQLConn) GetUserOrders(userID int64) ([]Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	f := func() (*sql.Rows, error) {
		rows, err := pg.DBConn.QueryContext(ctx, "SELECT * FROM ORDERS ORDER BY ID DESC WHERE user_id=$1", userID)
		if err != nil {
			return nil, err
		}
		return rows, nil
	}
	var netErr net.Error
	rows, err := retryReadFunc(2, 3, f, &netErr)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	var (
		orders []Order
		order  Order
	)
	for rows.Next() {
		err := rows.Scan(order.ID, order.Accrual, order.Date)
		if err != nil {
			pg.Logger.Error().Err(err).Msg("error in scanning rows")
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (pg *PGSQLConn) CheckLastID(table string) (ID int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	f := func() (*sql.Rows, error) {
		rows, err := pg.DBConn.QueryContext(ctx, "SELECT ID FROM $1 ORDER BY ID DESC LIMIT 1", strings.ToUpper(table))
		if err != nil {
			return nil, err
		}
		return rows, nil
	}
	var netErr net.Error
	rows, err := retryReadFunc(2, 3, f, &netErr)
	defer rows.Close()
	if err != nil {
		return 0, err
	}
	UserID := int64(0)
	for rows.Next() {
		err = rows.Scan(&UserID)
		if err != nil {
			return 0, err
		}
	}
	return ID, nil
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

func (pg *PGSQLConn) CheckUserCashback(userLogin string) (float64, float64, error) {
	rows, err := pg.makeQueryContext("SELECT cashback FROM USERS ORDER BY ID DESC WHERE login=$1", userLogin)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query userID from DB")
		return 0, 0, err
	}
	defer rows.Close()
	bal := Balance{}
	for rows.Next() {
		err := rows.Scan(bal.Current, bal.Withdrawn)
		if err != nil {
			return 0, 0, err
		}
	}
	return bal.Current, bal.Withdrawn, nil
}

func (pg *PGSQLConn) SubUserBalance(userLogin string, sum float64) (int64, error) {
	_, allTime, err := pg.CheckUserCashback(userLogin)
	if err != nil {
		return 0, err
	}
	rows, err := pg.makeExecContext("UPDATE USERS SET cashback=$1 cashback_all=$2 where login=$2", sum, userLogin, allTime+sum)
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) RegisterOrder(orderID int64, accrual float64, placedAt string, login string) (int64, error) {
	rows, err := pg.makeExecContext("INSERT INTO ORDERS (ID,accrual,placedAt,login) VALUES ($1,$2,$3,$4)", orderID, accrual, placedAt, login)
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) RegisterWithdraw(orderID int64, sum float64, placedAt string, login string) (int64, error) {
	rows, err := pg.makeExecContext("INSERT INTO WITHDRAWS (order_id, sum ,placedAt, login) VALUES ($1,$2,$3)", orderID, sum, placedAt, login)
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
	rows, err := pg.makeExecContext("INSERT INTO USERS (login,password,cashback,cashback_all) VALUES ($1,$2)", login, hashedPass, 0, 0)
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *PGSQLConn) CheckUserCreds(login string, plainPassword string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	f := func() (*sql.Rows, error) {
		rows, err := pg.DBConn.QueryContext(ctx, "SELECT PASSWORD FROM USERS WHERE LOGIN=$1", login)
		if err != nil {
			return nil, err
		}
		return rows, nil
	}
	var netErr net.Error
	rows, err := retryReadFunc(2, 3, f, &netErr)
	defer rows.Close()
	if err != nil {
		return false, err
	}
	var userHashedPwd []byte
	for rows.Next() {
		err = rows.Scan(&userHashedPwd)
		if err != nil {
			return false, err
		}
	}
	if CheckPasswordHash(userHashedPwd, []byte(plainPassword)) {
		return false, nil
	}
	return true, nil
}
