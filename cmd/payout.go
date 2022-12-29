package cmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/config"
	"github.com/nightowlcasino/nightowl/controller"
	logger "github.com/nightowlcasino/nightowl/logger"
	"github.com/nightowlcasino/nightowl/services/notif"
	"github.com/nightowlcasino/nightowl/services/payout"
	"github.com/nightowlcasino/nightowl/state"
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

			config.SetLoggingDefaults()

			if value := viper.Get("nats.endpoint"); value != nil {
				natsEndpoint = value.(string)
			} else {
				natsEndpoint = nats.DefaultURL
			}

			if value := viper.Get("nats.notif_payouts_subj"); value == nil {
				viper.Set("nats.notif_payouts_subj", "notif.payouts")
			}

			if value := viper.Get("payout.port"); value == nil {
				viper.Set("rng.port", "8090")
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

			t := &http.Transport{
				Dial: (&net.Dialer{
					Timeout: 3 * time.Second,
				}).Dial,
				MaxIdleConns:        100,
				MaxConnsPerHost:     100,
				MaxIdleConnsPerHost: 100,
				TLSHandshakeTimeout: 3 * time.Second,
			}

			retryClient := retryablehttp.NewClient()
			retryClient.HTTPClient.Transport = t
			retryClient.HTTPClient.Timeout = time.Second * 10
			retryClient.Logger = nil
			retryClient.RetryWaitMin = 200 * time.Millisecond
			retryClient.RetryWaitMax = 250 * time.Millisecond
			retryClient.RetryMax = 2
			retryClient.RequestLogHook = func(l retryablehttp.Logger, r *http.Request, i int) {
				retryCount := i
				if retryCount > 0 {
					log.Info("retryClient request failed, retrying...",
						zap.String("url", r.URL.String()),
						zap.Int("retryCount", retryCount),
					)
				}
			}

			var wg sync.WaitGroup

			notifState := state.NewNotifState(context.Background(), rdb)

			payoutSvc, err := payout.NewService(rdb, retryClient, notifState, &wg)
			if err != nil {
				log.Error("failed to create payout service", zap.Error(err))
				os.Exit(1)
			}

			notifSvc, err := notif.NewService(nc, rdb, retryClient, notifState, &wg)
			if err != nil {
				log.Error("failed to create notif service", zap.Error(err))
				os.Exit(1)
			}

			// populate NotifState from redis DB
			err = notifState.DBSync()
			if err != nil {
				log.Error("failed to sync redis DB for notif state", zap.Error(err))
				os.Exit(1)
			}

			router := controller.NewRouter(nc, rdb, "payout")
			server := controller.NewServer(router, viper.Get("payout.port").(int))

			server.Start()
			payoutSvc.Start()
			notifSvc.Start()

			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			go func() {
				s := <-signals
				log.Info(s.String() + " signal caught, stopping app")
				payoutSvc.Stop()
				notifSvc.Stop()
				server.Stop()
			}()

			log.Info("service started...")

			wg.Add(1)
			go payoutSvc.Wait(&wg)
			wg.Add(1)
			go notifSvc.Wait(&wg)
			go server.Wait()

			wg.Wait()
		},
	}
}