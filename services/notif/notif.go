package notif

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/erg"
	"github.com/nightowlcasino/nightowl/state"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	notConfirmedRedisKey = "confirmed:false"
)

var (
	log *zap.Logger
)

type Service struct {
	ctx       context.Context
	component string
	ergNode   *erg.ErgNode
	nats      *nats.Conn
	ns        *state.NotifState
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

func NewService(nats *nats.Conn, rdb *redis.Client, retryClient *retryablehttp.Client, ns *state.NotifState, wg *sync.WaitGroup) (service *Service, err error) {

	ctx := context.Background()
	log = zap.L()

	ergNodeClient, err := erg.NewErgNode(retryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create erg node client - %s", err.Error())
	}

	service = &Service{
		ctx:       ctx,
		component: "notif",
		ergNode:   ergNodeClient,
		nats:      nats,
		ns:        ns,
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

	checkPayouts <- true

loop:
	for {
		select {
		case <-stop:
			log.Info("stopping notifySpent() loop...")
			s.wg.Done()
			break loop
		case <-checkPayouts:
			// loop through and check if box id(s) are spent or not
			for notConf := range s.ns.NotConfirmed {
				firstColon := strings.Index(notConf, ":")
				lastColon := strings.LastIndex(notConf, ":")
				boxId := notConf[firstColon+1:lastColon]
				betType := notConf[:firstColon]
				// check if box id is spent
				log.Debug("checking boxId", zap.String("box_id", boxId))
				utxo, err := s.ergNode.GetErgUtxoBox(boxId)
				if err != nil {
					log.Error("failed to get utxo from node by box id", zap.Error(err), zap.String("box_id", boxId))
					continue
				}

				// if box id returned as spent (404 Not Found) then we build a Notif struct and send it
				// on the nats queue to be processed
				if utxo.BoxId == "" {
					log.Debug("boxId spent", zap.String("box_id", boxId))

					bet, err := s.rdb.HGetAll(s.ctx, notConf).Result()
					switch {
					case err != nil:
						log.Error("failed to get key from redis db", zap.Error(err), zap.String("redis_key", notConf))
						continue
					default:
						notif := Notif{
							Type: 		betType,
							WalletAddr: bet["playerAddr"],
							Amount: 	bet["winnerAmt"],
							TokenName: 	"OWL",
							TxID: 		bet["txId"],
						}
						notifMar, err := json.Marshal(notif)
						if err != nil {
							log.Error("failed to marshal notif struct", zap.Error(err), zap.Any("notif", notif))
							continue
						}
						err = s.nats.Publish(viper.Get("nats.notif_payouts_subj").(string), notifMar)
						if err != nil {
							log.Error("failed to publish notif struct to notif payouts subject",
								zap.Error(err),
								zap.Any("notif", notif),
								zap.String("nats_subject", viper.Get("nats.notif_payouts_subj").(string)),
							)
							continue
						}
						// remove tx from notif state data structure and confirmed:false redis set
						err = s.ns.RemoveNotConfirmed(notConf)
						if err != nil {
							log.Error("failed to remove member from redis db",
								zap.Error(err),
								zap.String("member_key", notConfirmedRedisKey),
								zap.String("member_name", notConf),
							)
							continue
						}
						// update redis db entry with confirmed = true
						addons := make(map[string]interface{})
						addons["confirmed"] = "true"

						err = s.rdb.HSet(s.ctx, notConf, addons).Err()
						if err != nil {
							log.Error("failed to set confirmed=true for bet in redis db", zap.Error(err), zap.String("redis_key", notConf))
						}
					}
				}
			}
		}

		go wait(30 * time.Second, checkPayouts)
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