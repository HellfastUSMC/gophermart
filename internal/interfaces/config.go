package interfaces

type Configurator interface {
	ParseStartupFlags() error
	GetDBPath() string
	GetServiceAddress() string
	GetCBPath() string
}
