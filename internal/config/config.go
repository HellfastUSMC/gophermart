package config

import (
	"flag"
	"os"

	"github.com/caarlos0/env/v6"
)

type SysConfig struct {
	GmartAddr     string `env:"LOCAL_ADDR"`
	CashbackAddr  string `env:"REMOTE_ADDR"`
	DBConnString  string `env:"DBCONN_STRING"`
	CheckInterval int64  `env:"CHECK_INTERVAL"`
	HashKey       string `env:"HASH_KEY"`
}

func (c *SysConfig) ParseStartupFlags() error {
	serverFlags := flag.NewFlagSet("server config", flag.ExitOnError)
	serverFlags.StringVar(&c.GmartAddr, "addr", "localhost:8080", "Address and port of server string")
	serverFlags.StringVar(
		&c.CashbackAddr,
		"rem",
		"localhost:8081",
		"Address and port of cashback service string",
	)
	serverFlags.Int64Var(&c.CheckInterval, "int", 600, "Checking interval in seconds int")
	serverFlags.StringVar(
		&c.DBConnString,
		"db",
		"postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
		"DB connection string",
	)
	serverFlags.StringVar(
		&c.HashKey,
		"hkey",
		"a01b02",
		"Hash key",
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
