package cmd

import (
	"os"
	"os/signal"
	"syscall"

	logger "github.com/nightowlcasino/nightowl/logger"
	"github.com/nightowlcasino/nightowl/services/payout"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// payoutSvcCommand is responsible for traversing the oracle addresses containing
// all the nightowl games bets and paying out the respective winner
func payoutSvcCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "payout-svc",
		Short: "Run a server that traverses the oracle addresses containing all the nightowl games bets and pay out the respective winner.",
		Run: func(_ *cobra.Command, _ []string) {

			logger.Initialize("no-payout-svc")
			log = zap.L()
			defer log.Sync()

			//if value := viper.Get("logging.level"); value != nil {
			//	lvl, err := logger.ParseLevel(value.(string))
			//	if err != nil {
			//		logger.Warnf(1, "config logging.level is not valid, defaulting to info log level")
			//		logger.SetVerbosity(1)
			//	}
			//	logger.SetVerbosity(lvl)
			//} else {
			//	logger.Warnf(1, "config logging.level is not found, defaulting to info log level")
			//	logger.SetVerbosity(1)
			//}

			if value := viper.Get("explorer_node.fqdn"); value == nil {
				viper.Set("explorer_node.fqdn", "api.ergoplatform.com")
			}

			if value := viper.Get("explorer_node.scheme"); value == nil {
				viper.Set("explorer_node.scheme", "https")
			}

			if value := viper.Get("explorer_node.port"); value == nil {
				viper.Set("explorer_node.port", 443)
			}

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
				log.Error("required config is absent", zap.Error(MissingNodeApiKeyErr))
				os.Exit(1)
			}

			if value := viper.Get("ergo_node.wallet_password"); value == nil {
				log.Error("required config is absent", zap.Error(MissingNodeWalletPassErr))
				os.Exit(1)
			}

			svc, err := payout.NewService()
			if err != nil {
				log.Error("failed to create payout service", zap.Error(err))
				os.Exit(1)
			}

			svc.Start()

			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func() {
				s := <-signals
				log.Info(s.String() + " signal caught, stopping app")
				svc.Stop()
			}()

			log.Info("service started...")

			svc.Wait()
		},
	}
}