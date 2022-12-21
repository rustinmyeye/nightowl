package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/hashicorp/go-multierror"
	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

var (
	notifTypes = []string{"swap","roulette"}
)

func SendNotifs(nc *nats.Conn, rdb *redis.Client) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		log := zap.L()
		start := time.Now()
		var count, failedCount int
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		walletAddr := params.ByName("walletAddr")
		reqURL := req.URL
		urlPath := reqURL.Path
		log.Debug("SendNotifs called",
			zap.String("url_path", urlPath),
			zap.String("wallet_addr", walletAddr),
		)

		// check if there are any pending notifications to send to the user from the redis db
		var errs *multierror.Error
		for _, typ := range notifTypes {
			match := fmt.Sprintf("notif:%s:%s:*", typ, walletAddr)
			iter := rdb.Scan(context.Background(), 0, match, 0).Iterator()
			for iter.Next(context.Background()) {
				n, err := rdb.Get(context.Background(), iter.Val()).Result()
				if err != nil {
					log.Error("failed to get notification from redis db",
						zap.Error(err),
						zap.String("redis_key", iter.Val()),
					)
					errs = multierror.Append(err, errs.Errors...)
				} else {
					// send notification to nats queue for user to consume
					err = nc.Publish("notif.payouts", []byte(n))
					if err != nil {
						log.Error("failed to send notification to nats queue",
							zap.Error(err),
							zap.String("wallet_addr", walletAddr),
						)
						errs = multierror.Append(err, errs.Errors...)
						failedCount++
					} else {
						count++
					}
				}
			}
			if err := iter.Err(); err != nil {
				log.Error("query failed to get notification from redis db",
					zap.Error(err),
					zap.String("redis_key", match),
					zap.String("wallet_addr", walletAddr),
				)
				errs = multierror.Append(err, errs.Errors...)
			}
		}

		if errs.ErrorOrNil() != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "{\"error\": \"failed to send some or all notification(s) to wallet address %s please try again\"}", walletAddr)
			log.Error("failed to send notification(s)",
				zap.Int64("durationMs",  time.Since(start).Milliseconds()),
				zap.String("wallet_addr", walletAddr),
				zap.Int("total_failed", failedCount),
			)
			return
		}

		log.Info("send notification(s) complete",
			zap.Int64("durationMs",  time.Since(start).Milliseconds()),
			zap.String("wallet_addr", walletAddr),
			zap.Int("total_sent", count),
		)
	
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{}")
	}
}