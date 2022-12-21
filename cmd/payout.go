package cmd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/go-redis/redis/v9"
	"github.com/nats-io/nats.go"
	logger "github.com/nightowlcasino/nightowl/logger"
	"github.com/nightowlcasino/nightowl/services/notif"
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

			logger.Initialize("no-payout-svc", hostname)
			log = zap.L()
			defer log.Sync()

			if value := viper.Get("logging.level"); value != nil {
				// logger will default to info level if user provided level is incorrect
				logger.SetLevel(value.(string))
			}

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
				log.Error("required config is absent", zap.Error(ErrMissingNodeApiKey))
				os.Exit(1)
			}

			if value := viper.Get("ergo_node.wallet_password"); value == nil {
				log.Error("required config is absent", zap.Error(ErrMissingNodeWalletPass))
				os.Exit(1)
			}

			// Connect to the nats server
			nc, err := nats.Connect(natsEndpoint)
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
			}

			payoutSvc, err := payout.NewService(rdb)
			if err != nil {
				log.Error("failed to create payout service", zap.Error(err))
				os.Exit(1)
			}

			notifSvc, err := notif.NewService(nc, rdb)
			if err != nil {
				log.Error("failed to create notif service", zap.Error(err))
				os.Exit(1)
			}

			payoutSvc.Start()
			notifSvc.Start()

			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func() {
				s := <-signals
				log.Info(s.String() + " signal caught, stopping app")
				payoutSvc.Stop()
				notifSvc.Stop()
			}()

			log.Info("service started...")

			var wg sync.WaitGroup

			wg.Add(1)
			go payoutSvc.Wait(&wg)
			wg.Add(1)
			go notifSvc.Wait(&wg)

			wg.Wait()
		},
	}
}