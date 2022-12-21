package config

import (
	"errors"
	"os"

	"github.com/nightowlcasino/nightowl/logger"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	Application     = "nightowl"
	ApplicationFull = "NightOwl Backend Services"
)

var (
	log *zap.Logger

	ErrMissingNodeWalletPass = errors.New("config ergo_node.wallet_password is missing")
	ErrMissingNodeApiKey = errors.New("config ergo_node.api_key is missing")
)

func SetLoggingDefaults() {
	if value := viper.Get("logging.level"); value != nil {
		// logger will default to info level if user provided level is incorrect
		logger.SetLevel(value.(string))
	} else {
		logger.SetLevel("info")
	}
}

func SetNodeDefaults() {
	log = zap.L()

	if value := viper.Get("ergo_node.fqdn"); value == nil {
		viper.Set("ergo_node.fqdn", "213.239.193.208")
	}

	if value := viper.Get("ergo_node.scheme"); value == nil {
		viper.Set("ergo_node.scheme", "http")
	}

	if value := viper.Get("ergo_node.port"); value == nil {
		viper.Set("ergo_node.port", 9053)
	}

	if value := viper.Get("ergo_node.api_key"); value == nil {
		log.Error("required config is absent", zap.Error(ErrMissingNodeApiKey))
		os.Exit(1)
	}

	if value := viper.Get("ergo_node.wallet_password"); value == nil {
		log.Error("required config is absent", zap.Error(ErrMissingNodeWalletPass))
		os.Exit(1)
	}
}

func SetExplorerDefaults() {
	if value := viper.Get("explorer_node.fqdn"); value == nil {
		viper.Set("explorer_node.fqdn", "api.ergoplatform.com")
	}

	if value := viper.Get("explorer_node.scheme"); value == nil {
		viper.Set("explorer_node.scheme", "https")
	}

	if value := viper.Get("explorer_node.port"); value == nil {
		viper.Set("explorer_node.port", 443)
	}
}