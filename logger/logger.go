package logger

import (
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

	loggerConf := zap.Config{
		Level: zap.NewAtomicLevelAt(zap.InfoLevel),
		Encoding: "json",
		EncoderConfig: ProdEncoderConf(),
		OutputPaths: []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger = zap.Must(loggerConf.Build())

	ljWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path+svc+".log",
  		MaxSize:    512, // megabytes
  		MaxBackups: 3,
  		MaxAge:     30,  // days
	})
	ljCore := zapcore.NewCore(zapcore.NewJSONEncoder(ProdEncoderConf()), ljWriteSyncer, zap.InfoLevel)

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

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func WithOptions(opts ...zap.Option) {
	defer zap.ReplaceGlobals(logger)
	logger = logger.WithOptions(opts...)
}
