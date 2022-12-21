package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-redis/redis/v9"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/controller"

	logger "github.com/nightowlcasino/nightowl/logger"
	"github.com/nightowlcasino/nightowl/services/notif"
	"github.com/nightowlcasino/nightowl/services/rng"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// rngSvcCommand is responsible for listening to frontend requests for a games
// random number which it gets from the nightowl oracle pool
func rngSvcCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rng-svc",
		Short: "Run a server that listens for frontend requests for a games random number which it obtains from nightowls oracle pool.",
		Run: func(_ *cobra.Command, _ []string) {

			logger.Initialize("no-rng-svc", hostname)
			log = zap.L()
			defer log.Sync()

			if value := viper.Get("logging.level"); value != nil {
				// logger will default to info level if user provided level is incorrect
				logger.SetLevel(value.(string))
			}

			// validate configs and set defaults if necessary
			if value := viper.Get("nats.endpoint"); value != nil {
				natsEndpoint = value.(string)
			} else {
				natsEndpoint = nats.DefaultURL
			}

			if value := viper.Get("nats.random_number_subj"); value == nil {
				viper.Set("nats.random_number_subj", "drand.hash")
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
				log.Error("required config is absent", zap.Error(ErrMissingNodeApiKey))
				os.Exit(1)
			}

			if value := viper.Get("ergo_node.wallet_password"); value == nil {
				log.Error("required config is absent", zap.Error(ErrMissingNodeWalletPass))
				os.Exit(1)
			}

			// Connect to the nats server
			nats, err := nats.Connect(natsEndpoint)
			if err != nil {
				log.Error("failed to connect to nats server", zap.Error(err), zap.String("endpoint", natsEndpoint))
				os.Exit(1)
			}

			// Connect to the redis db
			rdb := redis.NewClient(&redis.Options{
				Addr:	  "localhost:6379",
				Password: "",
				DB:		  0,
			})
			_, err = rdb.Ping(context.Background()).Result()
			if err != nil {
				log.Error("failed to connect to redis db", zap.Error(err), zap.String("endpoint", "localhost:6379"))
				os.Exit(1)
			}

			_, err = rng.NewService(nats)
			if err != nil {
				log.Error("failed to create rng service", zap.Error(err))
				os.Exit(1)
			}

			_, err = notif.NewService(nats, rdb)
			if err != nil {
				log.Error("failed to create rng service", zap.Error(err))
				os.Exit(1)
			}

			router := controller.NewRouter(nats, rdb)

			server := controller.NewServer(router)
			server.Start()
			
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func() {
				s := <-signals
				log.Info(s.String() + " signal caught, stopping app")
				server.Stop()
			}()

			log.Info("service started...")

			server.Wait()
		},
	}
}