package logger

import "github.com/rs/zerolog"

type Logger interface {
	Info() *zerolog.Event
	Warn() *zerolog.Event
	Error() *zerolog.Event
}
