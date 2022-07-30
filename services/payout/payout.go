package payout

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/nightowlcasino/nightowl/erg"
	log "github.com/sirupsen/logrus"
)

const (
	rouletteErgoTree    = "101b0400040004000402054a0e20473041c7e13b5f5947640f79f00d3c5df22fad4841191260350bb8c526f9851f040004000514052605380504050404020400040205040404050f05120406050604080509050c040a0e200ef2e4e25f93775412ac620a1da495943c55ea98e72f3e95d1a18d7ace2f676cd809d601b2a5730000d602b2db63087201730100d603b2db6501fe730200d604e4c672010404d605e4c6a70404d6069e7cb2e4c67203041a9a72047303007304d607e4c6a70504d6087e720705d6099972087206d1ed96830301938c7202017305938c7202028cb2db6308a77306000293b2b2e4c67203050c1a720400e4c67201050400c5a79597830601ed937205730795ec9072067308ed9272067309907206730a939e7206730b7208ed949e7206730c7208ec937207730d937207730eed937205730f939e720673107208eded937205731192720973129072097313ed9372057314939e720673157208eded937205731692720973179072097318ed9372057319937208720693c27201e4c6a7060e93cbc27201731a"
	houseAddress        = "ofgUTY7c693MfaVxfuZ1YhG7RQuQCLqa7mqFHkkZcpo9r5oPmmXaemS3raHAzfP4MXXc7DiueGDFsrZ5Hp3ZK"
	minerFee            = 1000000 // 0.0010 ERG
	minBoxValue         = 1000000 // 0.0010 ERG
)

type Service struct {
	ctx         context.Context
	component   string
	ergNode     *erg.ErgNode
	ergExplorer *erg.Explorer
	rdb         *redis.Client
	logger      *log.Entry
	stop        chan bool
	done        chan bool
}

func NewService(logger *log.Entry) (service *Service, err error) {

	ctx := context.Background()

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
			header := r.Header.Get("no-backend-func")
			logger.WithFields(log.Fields{"caller": header, "retryCount": retryCount}).Errorf("func call to %s failed retrying", header)
		}
	}

	ergExplorerClient, err := erg.NewExplorer(retryClient, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create erg explorer client - %s", err.Error())
	}

	ergNodeClient, err := erg.NewErgNode(retryClient, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create erg node client - %s", err.Error())
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:	  "localhost:6379",
		Password: "",
		DB:		  0,
	})

	service = &Service{
		ctx:         ctx,
		component:   "payout",
		ergNode:     ergNodeClient,
		ergExplorer: ergExplorerClient,
		rdb:         rdb,
		logger:      logger,
		stop:        make(chan bool),
		done:        make(chan bool),
	}

	return service, err
}

func (s *Service) payoutBets(stop chan bool) {
	var lastHeight int

	// get last known height stored in the redis db
	lastBetHeight, err := s.rdb.Get(s.ctx, "oracle:lastBetHeight").Result()
	switch {
	case err == redis.Nil || lastBetHeight == "":
		lastHeight = 0
		err := s.rdb.Set(s.ctx, "oracle:lastBetHeight", lastHeight, 0).Err()
		if err != nil {
			s.logger.WithFields(log.Fields{"error": err.Error()}).Errorf("failed to set key '%s' to redis db", "oracle:lastBetHeight")
		}
	case err != nil:
		s.logger.WithFields(log.Fields{"error": err.Error()}).Errorf("failed to get key '%s' from redis db", "oracle:lastBetHeight")
	default:
		lastHeight, _ = strconv.Atoi(lastBetHeight)
	}

loop:
	for {
		select {
		case <-stop:
			s.logger.Info("stopping payoutBets() loop...")
			break loop
		default:
			// clear structs
			var ergTxs = erg.ErgBoxIds{}
			var ergUtxo = erg.ErgTxOutputNode{}
			// Need to keep retrying if this fails
			currHeight, err := s.ergNode.GetCurrenHeight()
			if err != nil {
				s.logger.Errorf("failed to get current erg height - %s", err.Error())
			}

			start := time.Now()
			ergTxs, err = s.ergExplorer.GetOracleTxs(lastHeight, currHeight)
			if err != nil {
				s.logger.WithFields(log.Fields{"caller": "GetOracleTxs", "error": err.Error(), "durationMs": time.Since(start).Milliseconds()}).Error("failed to get oracle txs")
			} else {
				s.logger.WithFields(log.Fields{"caller": "GetOracleTxs", "durationMs": time.Since(start).Milliseconds()}).Infof("received %d txs to process", len(ergTxs.Items))
			}

			for _, ergTx := range ergTxs.Items {
				// convert R4 rendered value to []string
				r4 := ergTx.Outputs[0].AdditionalRegisters.R4.Value
				// remove surrounding brackets [ and ]
				r4 = strings.TrimPrefix(r4, "[")
				r4 = strings.TrimSuffix(r4, "]")
				ethHashSlices := strings.Split(r4, ",")

				// convert R5 rendered value to [][]string
				r5 := ergTx.Outputs[0].AdditionalRegisters.R5.Value
				// remove surrounding brackets [ and ]
				r5 = strings.TrimPrefix(r5, "[")
				r5 = strings.TrimSuffix(r5, "]")
				// add , to the back of string to help for the split
				r5 = r5 + ","
				ergBoxIdsSlices := strings.Split(r5, "],")
				// remove last element because it's empty
				ergBoxIdsSlices = ergBoxIdsSlices[:len(ergBoxIdsSlices)-1]

				for i, ergBoxIdsSlice := range ergBoxIdsSlices {
					// remove leading [
					ergBoxIdsClean := strings.TrimPrefix(ergBoxIdsSlice, "[")
					ergBoxIds := strings.Split(ergBoxIdsClean, ",")

					if len(ergBoxIds) > 0 && ergBoxIds[0] != "" {
						for j, boxId := range ergBoxIds {
							start := time.Now()
							ergUtxo, err = s.ergNode.GetErgUtxoBox(boxId)
							if err != nil {
								s.logger.WithFields(log.Fields{"caller": "GetErgUtxoBox", "error": err.Error(), "durationMs": time.Since(start).Milliseconds(), "erg_utxo_box_id": boxId}).Error("failed to get erg utxo box")
							} else {
								s.logger.WithFields(log.Fields{"caller": "GetErgUtxoBox", "durationMs": time.Since(start).Milliseconds(), "erg_utxo_box_id": boxId}).Debug("successfully got erg utxo box")
							}

							// check that bet ergoTree is the roulette house contract
							if ergUtxo.ErgoTree == rouletteErgoTree {
								startBet := time.Now()

								plyrAddr, _ := s.ergNode.ErgoTreeToAddress(ergUtxo.AdditionalRegisters.R6[4:])

								// check if bet exists in redis db
								bet, err := s.rdb.HGetAll(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr).Result()
								switch {
								case err == redis.Nil || len(bet) == 0:
									var randNum string
									if i+2 <= len(ethHashSlices)-1 {
										randNum = ethHashSlices[i+2]
									}

									b := make(map[string]string)
									b["settled"]    = "false"
									b["winnerAmt"]  = strconv.Itoa(ergUtxo.Assets[0].Amount)
									b["subgame"]    = ergUtxo.AdditionalRegisters.R4
									b["number"]     = ergUtxo.AdditionalRegisters.R5
									b["playerAddr"] = plyrAddr
									b["randomNum"]  = randNum

									// add bet to redis db
									err := s.rdb.HSet(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr, b).Err()
									if err != nil {
										s.logger.WithFields(log.Fields{"error": err.Error()}).Errorf("failed to set key '%s' to redis db", "roulette:"+ergUtxo.BoxId+":"+plyrAddr)
									}

									if randNum != "" {
										err := s.processBet(b, ergUtxo, ergTx, i, j)
										if err != nil {
											s.logger.WithFields(log.Fields{"error": err.Error()}).Error("failed to process bet")
										}
									}

								case err != nil:
									s.logger.WithFields(log.Fields{"error": err.Error()}).Errorf("failed to get key '%s' from redis db", "roulette:"+ergUtxo.BoxId+":"+plyrAddr)
								default:
									if bet["randomNum"] == "" {
										if i+2 <= len(ethHashSlices)-1 {
											err := s.rdb.HSet(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr, "randomNum", ethHashSlices[i+2]).Err()
											if err != nil {
												s.logger.WithFields(log.Fields{"error": err.Error()}).Errorf("failed to set key '%s' to redis db", "roulette:"+ergUtxo.BoxId+":"+plyrAddr)
											}
										}
									}

									// check if settled already
									isSettled, _ := strconv.ParseBool(bet["settled"])
									if !isSettled && bet["randomNum"] != "" {
										err := s.processBet(bet, ergUtxo, ergTx, i, j)
										if err != nil {
											s.logger.WithFields(log.Fields{"error": err.Error()}).Error("failed to process bet")
										}
										lastHeight = ergTx.Height
									}
								}
								s.logger.WithFields(log.Fields{"durationMs": time.Since(startBet).Milliseconds(), "erg_utxo_box_id": ergUtxo.BoxId}).Info("finished processing roulette bet")
							}
						}
					}
				}
			}
			time.Sleep(2 * time.Minute)
		}
	}
}

func (s *Service) Start() {
	
	stopPayout := make(chan bool)
	go s.payoutBets(stopPayout)

	// Wait for a "stop" message in the background to stop the service.
	go func(logger *log.Entry, stopPayout chan bool) {
		go func(logger *log.Entry) {
			<-s.stop
			stopPayout <- true
			s.done <- true
		}(s.logger)
	}(s.logger, stopPayout)
}

func (s *Service) Stop() {
	s.stop <- true
}

func (s *Service) Wait() {
	<-s.done
}

func (s *Service) processBet(bet map[string]string, box erg.ErgTxOutputNode, tx erg.ErgTx, boxPosX, boxPosY int) error {
	var winnerAddr string

	// figure out winner and create tx to send to result contract address
	randNum, err := getRandNum(bet["randomNum"])
	if err != nil {
		return fmt.Errorf("failed to get random number from key '%s' - %s", "roulette:"+box.BoxId+":"+bet["playerAddr"], err)
	} else {
		subgame, _ := strconv.Atoi(bet["subgame"][2:])
		chipspot, _ := strconv.Atoi(bet["number"][2:])
		sg := decodeZigZag64(uint64(subgame))
		cs := decodeZigZag64(uint64(chipspot))

		winner := winner(int(sg), int(cs), randNum)
		if winner {
			winnerAddr = bet["playerAddr"]
		} else {
			winnerAddr = houseAddress
		}
		s.logger.WithFields(log.Fields{"erg_utxo_box_id": box.BoxId, "winner_addr": winnerAddr, "random_number": randNum, "subgame": int(sg), "chipspot": int(cs)}).Debug("winner of roulette bet")

		serializedBetBox, _ := s.ergNode.SerializeErgBox(box.BoxId)
		serializedOracleBox, _ := s.ergNode.SerializeErgBox(tx.Outputs[0].BoxId)
		
		start := time.Now()
		txUnsigned, _ := buildResultSmartContractTx(box, encodeZigZag64(uint64(boxPosX)), encodeZigZag64(uint64(boxPosY)), winnerAddr, serializedBetBox, serializedOracleBox)
		s.logger.WithFields(log.Fields{"caller": "buildResultSmartContractTx", "durationMs": time.Since(start).Milliseconds(), "txUnsigned": string(txUnsigned)}).Debug("")
		
		start = time.Now()
		txSigned, err := s.ergNode.PostErgOracleTx(txUnsigned)
		if err != nil {
			s.logger.WithFields(log.Fields{"caller": "PostErgOracleTx", "durationMs": time.Since(start).Milliseconds()}).Error("")
			return fmt.Errorf("call to PostErgOracleTx failed - %s", err.Error())
		}
		s.logger.WithFields(log.Fields{"caller": "PostErgOracleTx", "durationMs": time.Since(start).Milliseconds(), "tx_id": string(txSigned)}).Info("successfully sent tx to result smart contract")
		
		// add tx id to the payout entry in redis
		tx_id := make(map[string]interface{})
		tx_id["txId"] = txSigned

		err = s.rdb.HSet(s.ctx, "roulette:"+box.BoxId+":"+bet["playerAddr"], tx_id).Err()
		if err != nil {
			return fmt.Errorf("failed to set txId for key '%s' to redis db - %s", "roulette:"+box.BoxId+":"+bet["playerAddr"], err)
		}

		// change lastBetHeight if we know we have successfully settled every bet which has
		// a height less than lastBetHeight
		err = s.rdb.Set(s.ctx, "oracle:lastBetHeight", tx.Height, 0).Err()
		if err != nil {
			return fmt.Errorf("failed to set key 'oracle:lastBetHeight' to redis db - %s", err)
		}
	}

	return nil
}

func buildResultSmartContractTx(betUtxo erg.ErgTxOutputNode, r4, r5 uint64, winnerAddress, betDataInput, oracleDataInput string) ([]byte, error) {
	// Build Erg Tx for node to sign
	var assets string

	if len(betUtxo.Assets) > 0 {
		// lenth of assets should only be 1 since we are only dealing with OWL tokens
		assets = fmt.Sprintf(`[{"tokenId": "%s", "amount": %d}]`, betUtxo.Assets[0].TokenId, betUtxo.Assets[0].Amount)
	}

	txToSign := []byte(fmt.Sprintf(`{
		"requests": [
			{
				"address": "%s",
				"value": %d,
				"assets": %s,
				"registers": {
					"R4": "04%02x",
					"R5": "04%02x"
				}
			}
		],
    	"fee": %d,
    	"inputsRaw": [
			"%s"
    	],
    	"dataInputsRaw": [
			"%s"
    	]
	}`, winnerAddress, minBoxValue, assets, r4, r5, minerFee, betDataInput, oracleDataInput))

	return txToSign, nil
}

func encodeZigZag64(n uint64) uint64 {
	return (n << 1) ^ (n >> 63)
}

func decodeZigZag64(n uint64) uint64 {
	return (n >> 1) ^ (-(n & 1))
}