package cmd

import (
	"os"

	"github.com/nightowlcasino/nightowl/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger       *log.Entry
	hostname     string
	natsEndpoint string
	cfgFile      string
	cmd = &cobra.Command{
		Use:   config.Application,
		Short: config.ApplicationFull,
		Long: `
Long Description`,
	}
)

// Execute is the core component for all the backend services
func Execute() error {
	return cmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)

	logger = log.WithField("appname", "no-core")

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yml)")
	viper.BindPFlag("config",cmd.Flags().Lookup("config"))
	viper.BindEnv("HOSTNAME")
	
	cmd.AddCommand(rngSvcCommand(logger))
	cmd.AddCommand(payoutSvcCommand(logger))
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

	logger = logger.WithFields(log.Fields{
		"hostname": hostname,
	})

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// use current directory
		dir, err := os.Getwd()
		if err != nil {
			logger.WithFields(log.Fields{"error": err.Error()}).Fatal("failed to get working directory")
		}
		viper.AddConfigPath(dir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := viper.ReadInConfig(); err == nil {
		logger.Infof("using config file: %s", viper.ConfigFileUsed())
	} else {
		logger.WithFields(log.Fields{"error": err.Error()}).Fatal("failed to read config file")
	}
}