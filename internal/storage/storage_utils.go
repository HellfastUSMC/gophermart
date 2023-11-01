package storage

import (
	"database/sql"

	"github.com/HellfastUSMC/gophermart/internal/logger"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
)

func PasswordHasher(plainPass string) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plainPass), bcrypt.DefaultCost)
	return bytes, err
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
