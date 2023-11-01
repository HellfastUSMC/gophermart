package config

import (
	"flag"
	"os"

	"github.com/caarlos0/env/v6"
)

type SysConfig struct {
	GmartAddr    string `env:"RUN_ADDRESS"`
	CashbackAddr string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DBConnString string `env:"DATABASE_URI"`
}

func (c *SysConfig) ParseStartupFlags() error {
	serverFlags := flag.NewFlagSet("server config", flag.ExitOnError)
	serverFlags.StringVar(&c.GmartAddr, "a", "localhost:8080", "Address and port of server string")
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
	if err := serverFlags.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
		return err
	}
	return nil
}

func newConfig() (*SysConfig, error) {
	config := SysConfig{}
	return &config, nil
}

func GetStartupConfigData() (*SysConfig, error) {
	conf, err := newConfig()
	if err != nil {
		return nil, err
	}
	err = conf.ParseStartupFlags()
	if err != nil {
		return nil, err
	}
	if err := env.Parse(conf); err != nil {
		return conf, err
	}
	return conf, nil
}
