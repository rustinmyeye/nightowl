package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/nightowlcasino/nightowl/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

	log *zap.Logger

	ErrMissingNodeWalletPass = errors.New("config ergo_node.wallet_password is missing")
	ErrMissingNodeApiKey = errors.New("config ergo_node.api_key is missing")
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
			fmt.Printf("unable to get hostname - %s\n", err.Error())
			os.Exit(1)
		}
	}

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// use current directory
		dir, err := os.Getwd()
		if err != nil {
			fmt.Printf("failed to get working directory - %s\n", err.Error())
			os.Exit(1)
		}
		viper.AddConfigPath(dir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := viper.ReadInConfig(); err == nil {
		fmt.Printf("using config file %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Printf("failed to read config file - %s\n", err.Error())
		os.Exit(1)
	}
}