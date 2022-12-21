package payout

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/nightowlcasino/nightowl/erg"
	"go.uber.org/zap"
)

const (
	rouletteErgoTree    = "101b0400040004000402054a0e20473041c7e13b5f5947640f79f00d3c5df22fad4841191260350bb8c526f9851f040004000514052605380504050404020400040205040404050f05120406050604080509050c040a0e200ef2e4e25f93775412ac620a1da495943c55ea98e72f3e95d1a18d7ace2f676cd809d601b2a5730000d602b2db63087201730100d603b2db6501fe730200d604e4c672010404d605e4c6a70404d6069e7cb2e4c67203041a9a72047303007304d607e4c6a70504d6087e720705d6099972087206d1ed96830301938c7202017305938c7202028cb2db6308a77306000293b2b2e4c67203050c1a720400e4c67201050400c5a79597830601ed937205730795ec9072067308ed9272067309907206730a939e7206730b7208ed949e7206730c7208ec937207730d937207730eed937205730f939e720673107208eded937205731192720973129072097313ed9372057314939e720673157208eded937205731692720973179072097318ed9372057319937208720693c27201e4c6a7060e93cbc27201731a"
	houseAddress        = "ofgUTY7c693MfaVxfuZ1YhG7RQuQCLqa7mqFHkkZcpo9r5oPmmXaemS3raHAzfP4MXXc7DiueGDFsrZ5Hp3ZK"
	minerFee            = 1000000 // 0.0010 ERG
	minBoxValue         = 1000000 // 0.0010 ERG
)

var (
	log *zap.Logger
)

type Service struct {
	ctx         context.Context
	component   string
	ergNode     *erg.ErgNode
	ergExplorer *erg.Explorer
	rdb         *redis.Client
	stop        chan bool
	done        chan bool
}

func NewService(rdb *redis.Client) (service *Service, err error) {

	ctx := context.Background()
	log = zap.L()

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

	ergExplorerClient, err := erg.NewExplorer(retryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create erg explorer client - %s", err.Error())
	}

	ergNodeClient, err := erg.NewErgNode(retryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create erg node client - %s", err.Error())
	}

	service = &Service{
		ctx:         ctx,
		component:   "payout",
		ergNode:     ergNodeClient,
		ergExplorer: ergExplorerClient,
		rdb:         rdb,
		stop:        make(chan bool),
		done:        make(chan bool),
	}

	return service, nil
}

func wait(sleepTime time.Duration, c chan bool) {
	time.Sleep(sleepTime)
	c <- true
}

func (s *Service) payoutBets(stop chan bool) {
	var lastHeight int
	checkbets := make(chan bool, 1)

	// get last known height stored in the redis db
	lastBetHeight, err := s.rdb.Get(s.ctx, "oracle:lastBetHeight").Result()
	switch {
	case err == redis.Nil || lastBetHeight == "":
		lastHeight = 0
		err := s.rdb.Set(s.ctx, "oracle:lastBetHeight", lastHeight, 0).Err()
		if err != nil {
			log.Error("failed to set key in redis db", zap.Error(err), zap.String("redis_key", "oracle:lastBetHeight"))
		}
	case err != nil:
		log.Error("failed to get key from redis db", zap.Error(err), zap.String("redis_key", "oracle:lastBetHeight"))
	default:
		lastHeight, _ = strconv.Atoi(lastBetHeight)
	}

	checkbets <- true

loop:
	for {
		select {
		case <-stop:
			log.Info("stopping payoutBets() loop...")
			break loop
		case <-checkbets:
			// clear structs
			var ergTxs = erg.ErgBoxIds{}
			var ergTxsBuff = erg.ErgBoxIds{}
			var ergUtxo = erg.ErgTxOutputNode{}
			var isSettled, allSettled bool
			var txHeight int
			limit := 50
			offset := 0
			// Need to keep retrying if this fails
			currHeight, err := s.ergNode.GetCurrenHeight()
			if err != nil {
				log.Error("failed to get current erg height", zap.Error(err))
			}

			// continuously call GetOracleTxs() until we get all txs
			start := time.Now()
			for {
				start1 := time.Now()
				ergTxsBuff, err = s.ergExplorer.GetOracleTxs(lastHeight, currHeight, limit, offset)
				if err != nil {
					log.Error("failed to get oracle txs",
						zap.Error(err),
						zap.Int64("durationMs",time.Since(start).Milliseconds()),
						zap.Int("last_height", lastHeight),
						zap.Int("curr_height", currHeight),
						zap.Int("limit", limit),
						zap.Int("offset", offset),
					)
					continue
				}
				log.Debug("received erg txs",
					zap.Int("tx_count", len(ergTxsBuff.Items)),
					zap.Int64("durationMs", time.Since(start1).Milliseconds()),
					zap.Int("last_height", lastHeight),
					zap.Int("curr_height", currHeight),
					zap.Int("limit", limit),
					zap.Int("offset", offset),
				)

				if len(ergTxsBuff.Items) == 0 {
					break
				}

				offset += limit
				ergTxs.Items = append(ergTxs.Items, ergTxsBuff.Items...)
			}
			log.Info("finished getting all oracle txs",
				zap.Int("total_txs", len(ergTxs.Items)),
				zap.Int64("durationMs", time.Since(start).Milliseconds()),
			)

			for _, ergTx := range ergTxs.Items {
				// assume all bets are settled until proven otherwise
				allSettled = true

				if ergTx.Height > txHeight {
					txHeight = ergTx.Height
				}
				// convert R4 rendered value to []string
				r4 := ergTx.Outputs[0].AdditionalRegisters.R4.Value
				// remove surrounding brackets [ and ]
				r4 = strings.TrimPrefix(r4, "[")
				r4 = strings.TrimSuffix(r4, "]")
				randNumbers := strings.Split(r4, ",")

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
							// check if stop signal was triggered or we are stuck in this loop
							select {
							case <-stop:
								log.Info("stopping payoutBets() loop...")
								break loop
							default:
								start := time.Now()
								ergUtxo, err = s.ergNode.GetErgUtxoBox(boxId)
								if err != nil {
									log.Error("failed to get erg utxo box",
										zap.Int64("durationMs", time.Since(start).Milliseconds()),
										zap.String("erg_utxo_box_id", boxId),
									)
								} else {
									log.Debug("successfully got erg utxo box",
										zap.Int64("durationMs", time.Since(start).Milliseconds()),
										zap.String("erg_utxo_box_id", boxId),
									)
								}

								// check that bet ergoTree is the roulette house contract
								if ergUtxo.ErgoTree == rouletteErgoTree {
									startBet := time.Now()

									plyrAddr, _ := s.ergNode.ErgoTreeToAddress(ergUtxo.AdditionalRegisters.R6[4:])

									// check if bet exists in redis db
									bet, err := s.rdb.HGetAll(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr).Result()
									switch {
									case err == redis.Nil || len(bet) == 0:
										isSettled = false
										var randNum string
										if i+1 <= len(randNumbers)-1 {
											randNum = randNumbers[i+1]
										}

										b := make(map[string]string)
										b["settled"]    = "false"
										b["winnerAmt"]  = strconv.Itoa(ergUtxo.Assets[0].Amount)
										b["winnerAddr"] = ""
										b["subgame"]    = ergUtxo.AdditionalRegisters.R4
										b["number"]     = ergUtxo.AdditionalRegisters.R5
										b["randomNum"]  = randNum

										// add bet to redis db
										err := s.rdb.HSet(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr, b).Err()
										if err != nil {
											log.Error("failed to set key in redis db", zap.Error(err), zap.String("redis_key", "roulette:"+ergUtxo.BoxId+":"+plyrAddr))
										}

										if randNum != "" {
											err := s.processBet(b, ergUtxo, ergTx, plyrAddr, i, j)
											if err != nil {
												log.Error("failed to process bet", zap.Error(err))
											} else {
												err = s.rdb.HSet(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr, "settled", "true").Err()
												if err != nil {
													log.Error("failed to set settled key to true in redis db", zap.Error(err), zap.String("redis_key", "roulette:"+ergUtxo.BoxId+":"+plyrAddr))
												}
												isSettled = true
											}
										}

									case err != nil:
										log.Error("failed to get key from redis db", zap.Error(err), zap.String("redis_key", "roulette:"+ergUtxo.BoxId+":"+plyrAddr))
									default:
										if bet["randomNum"] == "" {
											if i+1 <= len(randNumbers)-1 {
												bet["randomNum"] = randNumbers[i+1]
												err := s.rdb.HSet(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr, "randomNum", randNumbers[i+1]).Err()
												if err != nil {
													log.Error("failed to set key in redis db", zap.Error(err), zap.String("redis_key", "roulette:"+ergUtxo.BoxId+":"+plyrAddr))
												}
											}
										}

										// check if settled already
										isSettled, _ = strconv.ParseBool(bet["settled"])
										if !isSettled && bet["randomNum"] != "" {
											err := s.processBet(bet, ergUtxo, ergTx, plyrAddr, i, j)
											if err != nil {
												log.Error("failed to process bet", zap.Error(err))
											} else {
												err = s.rdb.HSet(s.ctx, "roulette:"+ergUtxo.BoxId+":"+plyrAddr, "settled", "true").Err()
												if err != nil {
													log.Error("failed to set settled key to true in redis db", zap.Error(err), zap.String("redis_key", "roulette:"+ergUtxo.BoxId+":"+plyrAddr))
												}
												isSettled = true
											}
										}
									}

									log.Info("finished processing roulette bet", zap.Int64("durationMs", time.Since(startBet).Milliseconds()), zap.String("erg_utxo_box_id", ergUtxo.BoxId))
								}
							}
						}
					}
					if !isSettled {
						allSettled = false
					}
				}
				if allSettled {
					// change lastBetHeight if we know we have successfully settled every bet which has
					// a height less than lastBetHeight
					if txHeight > lastHeight {
						err = s.rdb.Set(s.ctx, "oracle:lastBetHeight", txHeight, 0).Err()
						if err != nil {
							log.Error("failed to set key in redis db", zap.Error(err), zap.String("redis_key", "oracle:lastBetHeight"))
						}
						lastHeight = txHeight
					}
				}
			}
			
			// start timer in separate go routine
			go wait(2 * time.Minute, checkbets)
		}
	}
}

func (s *Service) Start() {
	
	stopPayout := make(chan bool)
	go s.payoutBets(stopPayout)

	// Wait for a "stop" message in the background to stop the service.
	go func(stopPayout chan bool) {
		go func() {
			<-s.stop
			stopPayout <- true
			s.done <- true
		}()
	}(stopPayout)
}

func (s *Service) Stop() {
	s.stop <- true
}

func (s *Service) Wait(wg *sync.WaitGroup) {
	defer wg.Done()
	<-s.done
}

func (s *Service) processBet(bet map[string]string, box erg.ErgTxOutputNode, tx erg.ErgTx, plyrAddr string, boxPosX, boxPosY int) error {
	var winnerAddr string

	// figure out winner and create tx to send to result contract address
	randNum, err := getRandNum(bet["randomNum"])
	if err != nil {
		return fmt.Errorf("failed to get random number from key '%s' - %s", "roulette:"+box.BoxId+":"+plyrAddr, err)
	} else {
		serializedBetBox, err := s.ergNode.SerializeErgBox(box.BoxId)
		if err != nil {
			return fmt.Errorf("call to SerializeErgBox with serializedBetBox failed - %s", err.Error())
		}
		serializedOracleBox, _ := s.ergNode.SerializeErgBox(tx.Outputs[0].BoxId)
		if err != nil {
			return fmt.Errorf("call to SerializeErgBox with serializedOracleBox failed - %s", err.Error())
		}

		subgame, _ := strconv.Atoi(bet["subgame"][2:])
		chipspot, _ := strconv.Atoi(bet["number"][2:])
		sg := decodeZigZag64(uint64(subgame))
		cs := decodeZigZag64(uint64(chipspot))

		winner := winner(int(sg), int(cs), randNum)
		if winner {
			winnerAddr = plyrAddr
		} else {
			winnerAddr = houseAddress
		}
		
		start := time.Now()
		txUnsigned, _ := buildResultSmartContractTx(box, encodeZigZag64(uint64(boxPosX)), encodeZigZag64(uint64(boxPosY)), winnerAddr, serializedBetBox, serializedOracleBox)
		log.Debug("unsigned erg tx created",
			zap.Int64("durationMs", time.Since(start).Milliseconds()),
			zap.String("txUnsigned", string(txUnsigned)),
		)
		
		start = time.Now()
		txSigned, err := s.ergNode.PostErgOracleTx(txUnsigned)
		if err != nil {
			log.Error("post erg tx failed", zap.Error(err), zap.Int64("durationMs", time.Since(start).Milliseconds()))
			return fmt.Errorf("call to PostErgOracleTx failed - %s", err.Error())
		}
		log.Info("successfully sent tx to result smart contract", zap.Int64("durationMs", time.Since(start).Milliseconds()), zap.String("tx_id", string(txSigned)))
		
		log.Debug("erg utxo box results",
			zap.String("erg_utxo_box_id", box.BoxId),
			zap.String("winner_addr", winnerAddr),
			zap.Int("winner_amount", box.Assets[0].Amount),
			zap.Int("random_number", randNum),
			zap.Int("subgame", int(sg)),
			zap.Int("chipspot", int(cs)),
		)

		// add tx id and winner address to the payout entry in redis
		addons := make(map[string]interface{})
		addons["txId"] = string(txSigned)
		addons["winnerAddr"] = string(winnerAddr)

		err = s.rdb.HSet(s.ctx, "roulette:"+box.BoxId+":"+plyrAddr, addons).Err()
		if err != nil {
			return fmt.Errorf("failed to set txId for key '%s' to redis db - %s", "roulette:"+box.BoxId+":"+plyrAddr, err)
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