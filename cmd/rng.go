package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/controller"
	"github.com/nightowlcasino/nightowl/logger"
	"github.com/nightowlcasino/nightowl/services/rng"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rngSvcCommand is responsible for listening to frontend requests for a games
// random number which it gets from the nightowl oracle pool
func rngSvcCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rng-svc",
		Short: "Run a server that listens for frontend requests for a games random number which it obtains from nightowls oracle pool.",
		Run: func(_ *cobra.Command, _ []string) {

			logger.SetDefaults(logger.Fields{
				"app": "no-rng-svc",
				"host": hostname,
			})

			if value := viper.Get("logging.level"); value != nil {
				lvl, err := logger.ParseLevel(value.(string))
				if err != nil {
					logger.Warnf(1, "config logging.level is not valid, defaulting to info log level")
					logger.SetVerbosity(1)
				}
				logger.SetVerbosity(lvl)
			} else {
				logger.Warnf(1, "config logging.level is not found, defaulting to info log level")
				logger.SetVerbosity(1)
			}

			// validate configs and set defaults if necessary
			if value := viper.Get("nats.endpoint"); value != nil {
				natsEndpoint = value.(string)
			} else {
				natsEndpoint = nats.DefaultURL
			}

			if value := viper.Get("rng.port"); value == nil {
				viper.Set("rng.port", "8089")
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
				logger.WithError(MissingNodeApiKeyErr).Infof(0, "required config is absent")
				os.Exit(1)
			}

			if value := viper.Get("ergo_node.wallet_password"); value == nil {
				logger.WithError(MissingNodeWalletPassErr).Infof(0, "required config is absent")
				os.Exit(1)
			}

			// Connect to the nats server
			nats, err := nats.Connect(natsEndpoint)
			if err != nil {
				logger.WithError(err).Infof(0, "failed to connect to %s nats server", natsEndpoint)
			}

			_, err = rng.NewService(nats)
			if err != nil {
				logger.WithError(err).Infof(0, "failed to create rng service")
			}

			router := controller.NewRouter(nats)

			server := controller.NewServer(router)
			server.Start()
			
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func() {
				s := <-signals
				logger.Infof(0, "%s signal caught, stopping app", s.String())
				server.Stop()
			}()

			logger.Infof(0, "service started...")

			server.Wait()
		},
	}
}