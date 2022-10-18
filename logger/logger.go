package logger

import (
	"os"

	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger *zap.Logger
	path string
)

type lumberjackSink struct {
	*lumberjack.Logger
}

func (lumberjackSink) Sync() error {
	return nil
}

func Initialize(svc string) {

	if value := viper.Get("LOG_PATH"); value != nil {
		path = value.(string)
	} else {
		path = "/var/log/"
	}

	logger = zap.New(zapcore.NewCore(
		zaplogfmt.NewEncoder(ProdEncoderConf()),
		os.Stdout,
		zap.NewAtomicLevelAt(zap.InfoLevel),
	), zap.AddCaller())

	ljWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path+svc+".log",
  		MaxSize:    512, // megabytes
  		MaxBackups: 3,
  		MaxAge:     30,  // days
	})

	ljCore := zapcore.NewCore(
		zaplogfmt.NewEncoder(ProdEncoderConf()),
		ljWriteSyncer,
		zap.NewAtomicLevelAt(zap.InfoLevel))

	logger = logger.WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core {
		return zapcore.NewTee(logger.Core(), ljCore)
	}))

	zap.ReplaceGlobals(logger)
}

func Flush() {
	if logger != nil {
		logger.Sync()
	}
}

func ProdEncoderConf() zapcore.EncoderConfig {
	encConf := zap.NewProductionEncoderConfig()
	encConf.EncodeTime = zapcore.RFC3339TimeEncoder

	return encConf
}
