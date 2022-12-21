package notif

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	log *zap.Logger
)

type Service struct {
	ctx       context.Context
	component string
	nats      *nats.Conn
	rdb       *redis.Client
	stop      chan bool
	done      chan bool
	wg        *sync.WaitGroup
}

type Notif struct {
	Type       string `json:"type"      redis:"type"`
	WalletAddr string `json:"address"   redis:"address"`
	Amount     string `json:"amount"    redis:"amount"`
	TokenName  string `json:"tokenName" redis:"tokenName"`
	TxID       string `json:"txid"      redis:"txid"`
}

func (n Notif) MarshalBinary() ([]byte, error) {
    return json.Marshal(n)
}

func NewService(nats *nats.Conn, rdb *redis.Client, wg *sync.WaitGroup) (service *Service, err error) {

	ctx := context.Background()
	log = zap.L()

	service = &Service{
		ctx:       ctx,
		component: "notif",
		nats:      nats,
		rdb:       rdb,
		stop:      make(chan bool),
		done:      make(chan bool),
		wg:        wg,
	}

	if _, err = nats.Subscribe(viper.Get("nats.notif_payouts_subj").(string), service.handleNATSMessages); err != nil {
		return nil, err
	}
	log.Info("successfully subscribed to " + viper.Get("nats.notif_payouts_subj").(string))

	return service, nil
}

func wait(sleepTime time.Duration, c chan bool) {
	time.Sleep(sleepTime)
	c <- true
}

func (s *Service) notifySpent(stop chan bool) {
	checkPayouts := make(chan bool, 1)
	// On start up load state from redis db
	// we are looking for txs that are settled == true but
	// confirmed == false and are not the liquidity pool wallet address
	
	// loop through and check if box ids are spent or not

	// if box id return as spent then we build a Notif struct and send it
	// on the nats queue to be processed

	// Lastly, update redis db entry with confirmed = true and then remove
	// tx from state data structure

	checkPayouts <- true

loop:
	for {
		select {
		case <-stop:
			log.Info("stopping notifySpent() loop...")
			s.wg.Done()
			break loop
		case <-checkPayouts:
			log.Debug("check payouts")
		}

		go wait(5 * time.Second, checkPayouts)
	}
}

func (s *Service) Start() {
	
	stopScanning := make(chan bool)
	s.wg.Add(1)
	go s.notifySpent(stopScanning)

	// Wait for a "stop" message in the background to stop the service.
	go func(stopScanning chan bool) {
		go func() {
			<-s.stop
			stopScanning <- true
			s.done <- true
		}()
	}(stopScanning)
}

func (s *Service) Stop() {
	s.stop <- true
}

func (s *Service) Wait(wg *sync.WaitGroup) {
	defer wg.Done()
	<-s.done
}

// handleNATSMessages is called on receipt of a new NATS message.
func (s *Service) handleNATSMessages(msg *nats.Msg) {
	var notif Notif
	err := json.Unmarshal(msg.Data, &notif)
	if err != nil {
		log.Error("failed to unmarshal Notif", zap.Error(err))
	} else {
		// attempt to send notification(s) to wallet address
		subj := fmt.Sprintf("notif.%s", notif.WalletAddr)
		inbox := nats.NewInbox()
		reply, err := s.nats.SubscribeSync(inbox)
		if err != nil {
			log.Error("failed to subscribe to inbox subject", zap.Error(err), zap.String("inbox_subject", inbox))
			return
		}
		reply.AutoUnsubscribe(1)

		err = s.nats.PublishRequest(subj, inbox, msg.Data)
		if err != nil {
			log.Error("failed to publish notification to subject", zap.Error(err), zap.String("subject", subj))
			return
		}

		log.Debug("sent notification to player",
			zap.String("type", notif.Type),
			zap.String("wallet_addr", notif.WalletAddr),
			zap.String("amount", notif.Amount),
			zap.String("token_name", notif.TokenName),
			zap.String("tx_id", notif.TxID),
		)
		_, err = reply.NextMsg(10 * time.Second)
		if err != nil {
			// if no ack received then we add it to redis db until player reconnects
			log.Debug("notification ack not received",
				zap.Error(err),
				zap.String("type", notif.Type),
				zap.String("wallet_addr", notif.WalletAddr),
				zap.String("amount", notif.Amount),
				zap.String("token_name", notif.TokenName),
				zap.String("tx_id", notif.TxID),
			)
			key := fmt.Sprintf("notif:%s:%s:%s", notif.Type, notif.WalletAddr, notif.TxID)
			result, err := s.rdb.Get(s.ctx, key).Result()
			switch {
			case err == redis.Nil || result == "":
				// store unsent notifications for 2 weeks
				err = s.rdb.Set(s.ctx, key, notif, 336 * time.Hour).Err()
				if err != nil {
					log.Error("failed to set key in redis db", zap.Error(err), zap.String("redis_key", key))
				} else {
					log.Debug("notification stored in redis db", zap.String("redis_key", key))
				}
			case err != nil:
				log.Error("failed to get key from redis db", zap.Error(err), zap.String("redis_key", key))
			}
		} else {
			key := fmt.Sprintf("notif:%s:%s:%s", notif.Type, notif.WalletAddr, notif.TxID)
			log.Debug("notification ack received, removing notification from redis db", zap.String("redis_key", key))
			_, err := s.rdb.Del(s.ctx, key).Result()
			if err != nil {
				log.Error("failed to remove notification from redis db", zap.Error(err), zap.String("redis_key", key))
			}
		}
	}
}