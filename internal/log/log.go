package log

import (
	"go.uber.org/zap"
)

var defaultLogger = zap.NewNop()

func Get() *zap.Logger {
	return defaultLogger
}

func Set() {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var err error
	defaultLogger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
}

func Flush() {
	_ = defaultLogger.Sync()
}
