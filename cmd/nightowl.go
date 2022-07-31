package cmd

import (
	"errors"
	"os"

	"github.com/nightowlcasino/nightowl/config"
	"github.com/nightowlcasino/nightowl/logger"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	hostname     string
	natsEndpoint string
	cfgFile      string
	cmd = &cobra.Command{
		Use:   config.Application,
		Short: config.ApplicationFull,
		Long: `
Long Description`,
	}

	MissingNodeWalletPassErr = errors.New("config ergo_node.wallet_password is missing")
	MissingNodeApiKeyErr = errors.New("config ergo_node.api_key is missing")
)

// Execute is the core component for all the backend services
func Execute() error {
	return cmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yml)")
	viper.BindPFlag("config",cmd.Flags().Lookup("config"))
	viper.BindEnv("HOSTNAME")
	
	cmd.AddCommand(rngSvcCommand())
	cmd.AddCommand(payoutSvcCommand())
}

func initConfig() {

	if value := viper.Get("HOSTNAME"); value != nil {
		hostname = value.(string)
	} else {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			log.Fatal("unable to get hostname")
		}
	}

	logger.SetDefaults(logger.Fields{
		"host": hostname,
	})

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// use current directory
		dir, err := os.Getwd()
		if err != nil {
			logger.WithError(err).Infof(0, "failed to get working directory")
			os.Exit(1)
		}
		viper.AddConfigPath(dir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := viper.ReadInConfig(); err == nil {
		logger.Infof(0, "using config file: %s", viper.ConfigFileUsed())
	} else {
		logger.WithError(err).Infof(0, "failed to read config file")
		os.Exit(1)
	}
}