// backend/pkg/logger/zap.go
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.SugaredLogger

func Init(isDev bool) {
	var cfg zap.Config
	if isDev {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	Log = logger.Sugar()
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
