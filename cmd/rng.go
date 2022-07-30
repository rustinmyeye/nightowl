package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/controller"
	"github.com/nightowlcasino/nightowl/services/rng"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rngSvcCommand is responsible for listening to frontend requests for a games
// random number which it gets from the nightowl oracle pool
func rngSvcCommand(logger *log.Entry) *cobra.Command {
	return &cobra.Command{
		Use:   "rng-svc",
		Short: "Run a server that listens for frontend requests for a games random number which it obtains from nightowls oracle pool.",
		Run: func(_ *cobra.Command, _ []string) {

			logger = logger.WithFields(log.Fields{
				"appname": "no-rng-svc",
				"hostname": hostname,
			})

			if value := viper.Get("logging.level"); value != nil {
				lvl, err := log.ParseLevel(value.(string))
				if err != nil {
					logger.Warn("config logging.level is not valid, defaulting to info log level")
				}
				log.SetLevel(lvl)
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
				logger.Fatal("config ergo_node.api_key is missing and is required")
			}

			if value := viper.Get("ergo_node.wallet_password"); value == nil {
				logger.Fatal("config ergo_node.wallet_password is missing and is required")
			}

			// Connect to the nats server
			nats, err := nats.Connect(natsEndpoint)
			if err != nil {
				logger.WithFields(log.Fields{"error": err.Error()}).Fatal("failed to connect to ':4222' nats server")
			}

			_, err = rng.NewService(logger, nats)
			if err != nil {
				logger.WithFields(log.Fields{"error": err.Error()}).Fatal("failed to create rng service")
			}

			router := controller.NewRouter(logger, nats)

			server := controller.NewServer(router)
			server.Start()
			
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func(logger *log.Entry) {
				s := <-signals
				logger.Infof("%s signal caught, stopping app", s.String())
				server.Stop()
			}(logger)

			logger.Info("service started...")

			server.Wait()
		},
	}
}