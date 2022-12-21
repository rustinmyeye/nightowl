package controller

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/services/rng"
	"go.uber.org/zap"
)

const (
	hexBytes = "0123456789abcdef"
)

var (
	ErrRandNumNotFound = errors.New("timeout - random number not found")
)

func wait(sleepTime time.Duration, c chan bool) {
	time.Sleep(sleepTime)
	c <- true
}

func random(n int, src rand.Source) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = hexBytes[src.Int63()%int64(len(hexBytes))]
	}
	return string(b)
}

func SendRandNum(nc *nats.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		log := zap.L()
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		sessionId := req.Header.Get("owl-session-id")
		boxId := req.URL.Query().Get("boxId")
		walletAddr := req.URL.Query().Get("walletAddr")
		game := params.ByName("game")
		reqURL := req.URL
		urlPath := reqURL.Path
		log.Info("sendRandNum called",
			zap.String("url_path", urlPath),
			zap.String("box_id", boxId),
			zap.String("game", game),
			zap.String("wallet_addr", walletAddr),
			zap.String("session_id", sessionId),
		)

		go func(game, boxId, walletAddr string, nc *nats.Conn) {
			timeout := time.NewTicker(120 * time.Second)
			wake := make(chan bool, 1)
			// let it select the wake case first
			wake <- true

			for {
				select {
				case <-timeout.C:
					log.Info("sendRandNum timed out",
						zap.Error(ErrRandNumNotFound),
						zap.Int64("durationMs", time.Since(start).Milliseconds()),
						zap.String("box_id", boxId),
						zap.String("game", game),
						zap.String("wallet_addr", walletAddr),
						zap.String("session_id", sessionId),
					)
					return
				case <-wake:
					if randNum, ok := rng.GetRandHashMap().Get(boxId); ok {
						topic := fmt.Sprintf("%s.%s", game, walletAddr)
						nc.Publish(topic, []byte(randNum))
						log.Info("successfully sent random number",
							zap.Int64("durationMs",  time.Since(start).Milliseconds()),
							zap.String("rand_num", randNum),
							zap.String("box_id", boxId),
							zap.String("game", game),
							zap.String("wallet_addr", walletAddr),
							zap.String("session_id", sessionId),
						)
						return
					}
				}
				go wait(5 * time.Second, wake)
			}
		}(game, boxId, walletAddr, nc)
	
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
	}
}

func SendTestRandNum(nc *nats.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		log := zap.L()
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		sessionId := req.Header.Get("owl-session-id")
		walletAddr := req.URL.Query().Get("walletAddr")
		reqURL := req.URL
		urlPath := reqURL.Path
		log.Info("SendTestRandNum called",
			zap.Int64("durationMs", time.Since(start).Milliseconds()),
			zap.String("url_path", urlPath),
			zap.String("game", "roulette"),
			zap.String("wallet_addr", walletAddr),
			zap.String("session_id", sessionId),
		)

		go func(walletAddr string, nc *nats.Conn) {
			randSrc := rand.NewSource(time.Now().UnixNano())
			randNum := random(8, randSrc)

			wake := make(chan bool, 1)
			go wait(10 * time.Second, wake)

			for range wake {
				topic := fmt.Sprintf("roulette.%s", walletAddr)
				nc.Publish(topic, []byte(randNum))
				log.Info("successfully sent random number",
					zap.Int64("durationMs", time.Since(start).Milliseconds()),
					zap.String("rand_num", randNum),
					zap.String("game", "roulette"),
					zap.String("wallet_addr", walletAddr),
					zap.String("session_id", sessionId),
				)
				return
			}
		}(walletAddr, nc)
	
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
	}
}