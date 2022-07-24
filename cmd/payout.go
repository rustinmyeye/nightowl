package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/nightowlcasino/nightowl/services/payout"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// payoutSvcCommand is responsible for traversing the oracle addresses containing
// all the nightowl games bets and paying out the respective winner
func payoutSvcCommand(logger *log.Entry) *cobra.Command {
	return &cobra.Command{
		Use:   "payout-svc",
		Short: "Run a server that traverses the oracle addresses containing all the nightowl games bets and pay out the respective winner.",
		Run: func(_ *cobra.Command, _ []string) {
			
			logger = logger.WithFields(log.Fields{
				"appname": "no-payout-svc",
				"hostname": hostname,
			})

			if value := viper.Get("explorer_node.fqdn"); value == nil {
				viper.Set("explorer_node.fqdn", "ergo-explorer-cdn.getblok.io")
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
				logger.Fatal("config ergo_node.api_key is missing and is required")
			}

			if value := viper.Get("ergo_node.wallet_password"); value == nil {
				logger.Fatal("config ergo_node.wallet_password is missing and is required")
			}

			svc, err := payout.NewService(logger)
			if err != nil {
				logger.WithFields(log.Fields{"error": err.Error()}).Fatal("failed to create payout service")
			}

			svc.Start()

			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func(logger *log.Entry) {
				s := <-signals
				logger.Infof("%s signal caught, stopping app", s.String())
				svc.Stop()
			}(logger)

			logger.Info("service started...")

			svc.Wait()
		},
	}
}