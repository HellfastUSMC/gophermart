package config

import (
	"flag"
	"os"

	"github.com/caarlos0/env/v6"
)

type SysConfig struct {
	GmartAddr      string `env:"RUN_ADDRESS"`
	CashbackAddr   string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DBConnString   string `env:"DATABASE_URI"`
	TokensInterval int64  `env:"T_INTERVAL"`
	HealthInterval int64  `env:"H_INTERVAL"`
	OrdersInterval int64  `env:"O_INTERVAL"`
}

func (c *SysConfig) ParseStartupFlags() error {
	serverFlags := flag.NewFlagSet("server config", flag.ExitOnError)
	serverFlags.StringVar(
		&c.GmartAddr,
		"a",
		"localhost:8080",
		"Address and port of server string",
	)
	serverFlags.StringVar(
		&c.CashbackAddr,
		"r",
		"localhost:8081",
		"Address and port of cashback service string",
	)
	serverFlags.StringVar(
		&c.DBConnString,
		"d",
		"postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
		"DB connection string",
	)
	serverFlags.Int64Var(
		&c.TokensInterval,
		"ti",
		1,
		"Tokens check interval in hours int64",
	)
	serverFlags.Int64Var(
		&c.HealthInterval,
		"hi",
		1,
		"Health check interval in hours int64",
	)
	serverFlags.Int64Var(
		&c.OrdersInterval,
		"oi",
		1,
		"Orders check interval in seconds int64",
	)
	if err := serverFlags.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
		return err
	}
	return nil
}
func (c *SysConfig) GetCBPath() string {
	return c.CashbackAddr
}
func (c *SysConfig) GetDBPath() string {
	return c.DBConnString
}
func (c *SysConfig) GetServiceAddress() string {
	return c.GmartAddr
}
func newConfig() *SysConfig {
	return &SysConfig{}
}

func GetStartupConfigData() (*SysConfig, error) {
	conf := newConfig()
	err := conf.ParseStartupFlags()
	if err != nil {
		return nil, err
	}
	if err := env.Parse(conf); err != nil {
		return conf, err
	}
	return conf, nil
}
