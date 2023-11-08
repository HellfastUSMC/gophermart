package dbconnector

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	"github.com/HellfastUSMC/gophermart/internal/storage"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type SQLConn struct {
	ConnectionString string
	DBConn           *sql.DB
	Logger           logger.Logger
	SQLUserOps
	SQLOrderOps
	SQLBonusOps
}

type SQLUserOps struct {
	Logger logger.Logger
	DBConn *sql.DB
}
type SQLOrderOps struct {
	Logger logger.Logger
	DBConn *sql.DB
}
type SQLBonusOps struct {
	Logger logger.Logger
	DBConn *sql.DB
}

func (pg *SQLConn) Close() error {
	err := pg.DBConn.Close()
	if err != nil {
		return err
	}
	return nil
}

func makeQueryContext(dbConn *sql.DB, query string, args ...any) (*sql.Rows, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	f := func() (*sql.Rows, error) {
		rows, err := dbConn.QueryContext(ctx, query, args...)
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

func makeExecContext(dbConn *sql.DB, logger logger.Logger, query string, args ...any) (int64, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	res, err := dbConn.ExecContext(ctx, query, args...)
	if err != nil {
		logger.Error().Err(err).Msg("error when query from DB")
		return 0, cancel, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		logger.Error().Err(err).Msg("error when get rows affected")
		return 0, cancel, err
	}
	return rows, cancel, nil
}

//func (pg *SQLUserOps) GetUserBalance(login string) (float64, float64, error) {
//	balance, cancel := makeQueryRowCTX(pg.DBConn, "SELECT cashback FROM USERS WHERE login=$1", login)
//	defer cancel()
//	var balance float64
//	var withdrawn float64
//	err := row.Scan(&balance, &withdrawn)
//	if err != nil {
//		pg.Logger.Error().Err(err).Msg("error when scanning rows")
//		return 0, 0, err
//	}
//	return balance, withdrawn, nil
//}

func (pg *SQLConn) GetUserWithdrawals(login string) ([]storage.Bonus, error) {
	rows, cancel, err := makeQueryContext(pg.DBConn, "SELECT id,order_id,sum,placed_at,login FROM BONUSES WHERE login=$1 AND sub=true ORDER BY placed_at", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when query user withdraws from DB")
		return nil, err
	}
	var (
		withdrawals []storage.Bonus
		withdraw    storage.Bonus
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

func makeQueryRowCTX(dbConn *sql.DB, query string, args ...any) (*sql.Row, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	row := dbConn.QueryRowContext(ctx, query, args...)
	return row, cancel
}

func (pg *SQLConn) GetOrder(order string) (storage.Order, error) {
	row, cancel := makeQueryRowCTX(pg.DBConn, "SELECT * FROM ORDERS WHERE id=$1", order)
	defer cancel()
	var ord storage.Order
	err := row.Scan(&ord.ID, &ord.Accrual, &ord.Date, &ord.Login, &ord.Status)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when scanning row")
		return ord, err
	}
	return ord, nil
}

func (pg *SQLOrderOps) GetUserOrders(login string) ([]storage.Order, error) {
	rows, cancel, err := makeQueryContext(pg.DBConn, "SELECT * FROM ORDERS WHERE login=$1 ORDER BY placed_at", login)
	if err != nil {
		pg.Logger.Error().Err(err).Msg("error when searching user orders in DB")
		return nil, err
	}
	defer cancel()
	var (
		orders []storage.Order
		order  storage.Order
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

func (pg *SQLConn) Ping() error {
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

func (pg *SQLUserOps) CheckUserBalance(userLogin string) (float64, float64, error) {
	current, cancel1 := makeQueryRowCTX(pg.DBConn, "SELECT cashback FROM USERS WHERE login=$1", userLogin)
	defer cancel1()
	withdrawn, cancel2 := makeQueryRowCTX(pg.DBConn, "SELECT SUM(SUM) FROM BONUSES WHERE login=$1 AND sub=true", userLogin)
	defer cancel2()
	var withdrawnDB sql.NullFloat64
	var currentDB sql.NullFloat64
	err := current.Scan(&currentDB)
	if err != nil {
		return 0, 0, err
	}
	err = withdrawn.Scan(&withdrawnDB)
	if err != nil {
		return 0, 0, err
	}
	return currentDB.Float64, withdrawnDB.Float64, nil
}

func (pg *SQLUserOps) UpdateUserBalance(checkUserBalance func(string) (float64, float64, error), userLogin string, sum float64, sub bool) (int64, error) {
	if sub {
		current, _, err := checkUserBalance(userLogin)
		if err != nil {
			return 0, err
		}
		if current < sum {
			return 0, fmt.Errorf("can't substract from balance, current=%f < order=%f", current, sum)
		}
		rows, cancel, err := makeExecContext(pg.DBConn, pg.Logger, "UPDATE USERS SET cashback=cashback-$1 WHERE login=$2",
			sum, userLogin,
		)
		defer cancel()
		if err != nil {
			return 0, err
		}
		return rows, nil
	}
	rows, cancel, err := makeExecContext(
		pg.DBConn,
		pg.Logger,
		"UPDATE USERS SET cashback=cashback+$1 WHERE login=$2",
		sum, userLogin,
	)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *SQLOrderOps) RegisterOrder(orderID string, accrual float64, placedAt string, login string) (int64, error) {
	rows, cancel, err := makeExecContext(
		pg.DBConn,
		pg.Logger,
		"INSERT INTO ORDERS (id,cashback,placed_at,login,status) VALUES ($1,$2,$3,$4,$5)",
		orderID, accrual, placedAt, login, "NEW",
	)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *SQLOrderOps) UpdateOrder(orderID string, accrual float64, status string) (int64, error) {
	rows, cancel, err := makeExecContext(
		pg.DBConn,
		pg.Logger,
		"UPDATE ORDERS SET cashback=$1, status=$3 WHERE id=$2",
		accrual, orderID, status,
	)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *SQLBonusOps) RegisterBonusChange(orderID string, sum float64, placedAt string, login string, sub bool) (int64, error) {
	rows, cancel, err := makeExecContext(
		pg.DBConn,
		pg.Logger,
		"INSERT INTO BONUSES (order_id, sum ,placed_at, login, sub) VALUES ($1,$2,$3,$4,$5)",
		orderID, sum, placedAt, login, sub,
	)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *SQLUserOps) RegisterUser(login string, password string) (int64, error) {
	hashedPass, err := storage.PasswordHasher(password)
	if err != nil {
		return 0, err
	}
	rows, cancel, err := makeExecContext(
		pg.DBConn,
		pg.Logger,
		"INSERT INTO USERS (login,password,cashback) VALUES ($1,$2,$3)",
		login, string(hashedPass), 0,
	)
	defer cancel()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (pg *SQLUserOps) CheckUserCreds(login string, plainPassword string) (bool, error) {
	row, cancel := makeQueryRowCTX(
		pg.DBConn,
		"SELECT password FROM USERS WHERE LOGIN=$1",
		login,
	)
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

func (pg *SQLOrderOps) GetOrdersToCheck() ([]storage.Order, error) {
	rows, cancel, err := makeQueryContext(
		pg.DBConn,
		"SELECT * FROM orders WHERE status!='INVALID' AND status!='PROCESSED'",
	)
	defer cancel()
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var orders []storage.Order
	for rows.Next() {
		var order storage.Order
		err = rows.Scan(&order.ID, &order.Accrual, &order.Date, &order.Login, &order.Status)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func NewConnectionSQL(connPath string, logger logger.Logger) (*SQLConn, error) {
	db, err := sql.Open("pgx", connPath)
	if err != nil {
		return nil, err
	}
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return nil, err
	}
	user := SQLUserOps{
		DBConn: db,
		Logger: logger,
	}
	order := SQLOrderOps{
		DBConn: db,
		Logger: logger,
	}
	bonus := SQLBonusOps{
		DBConn: db,
		Logger: logger,
	}
	return &SQLConn{
		connPath,
		db,
		logger,
		user,
		order,
		bonus,
	}, nil
}
