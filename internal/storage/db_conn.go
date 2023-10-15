package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/HellfastUSMC/gophermart/gophermart/internal/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
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

func (pg *PGSQLConn) CheckUserCreds(login string, password string) bool {
	return true
}

func NewConnectionPGSQL(connPath string, logger logger.CLogger) (*PGSQLConn, error) {
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
	return &PGSQLConn{
		ConnectionString: connPath,
		DBConn:           db,
		Logger:           logger,
	}, nil
}
