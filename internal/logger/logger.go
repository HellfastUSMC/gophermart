package logger

import "github.com/rs/zerolog"

type CLogger interface {
	Info() *zerolog.Event
	Warn() *zerolog.Event
	Error() *zerolog.Event
}
