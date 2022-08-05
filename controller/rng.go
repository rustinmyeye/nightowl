package controller

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/logger"
	"github.com/nightowlcasino/nightowl/services/rng"
)

const (
	// HeaderContentType is the Content-Type header key.
	HeaderContentType = "Content-Type"
	// ContentTypeJSON is the application/json MIME type.
	ContentTypeJSON = "application/json"

	hexBytes = "0123456789abcdef"
)

var (
	RandNumNotFound = errors.New("timeout - random number not found")
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

func opts() httprouter.Handle {
	return func (w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, owl-session-id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
	}
}

func SendRandNum(nc *nats.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		sessionId := req.Header.Get("owl-session-id")
		boxId := req.URL.Query().Get("boxId")
		walletAddr := req.URL.Query().Get("walletAddr")
		game := mux.Vars(req)["game"]
		reqURL := req.URL
		urlPath := reqURL.Path
		logger.WithFields(logger.Fields{
			"caller":      "sendRandNum",
			"url_path":    urlPath,
			"box_id":      boxId,
			"game":        game,
			"wallet_addr": walletAddr,
			"session_id":  sessionId,
		}).Infof(0, "sendRandNum called")

		go func(game, boxId, walletAddr string, nc *nats.Conn) {
			timeout := time.NewTicker(120 * time.Second)
			for {
				select {
				case <-timeout.C:
					logger.WithError(RandNumNotFound).WithFields(logger.Fields{
						"caller":      "sendRandNum",
						"durationMs":  time.Since(start).Milliseconds(),
						"box_id":      boxId,
						"game":        game,
						"wallet_addr": walletAddr,
						"session_id":  sessionId,
					}).Infof(0, "")
					return
				default:
					if randNum, ok := rng.GetRandHashMap().Get(boxId); ok {
						topic := fmt.Sprintf("%s.%s", game, walletAddr)
						nc.Publish(topic, []byte(randNum))
						logger.WithFields(logger.Fields{
							"caller":      "sendRandNum",
							"durationMs":  time.Since(start).Milliseconds(),
							"rand_num":    randNum,
							"box_id":      boxId,
							"game":        game,
							"wallet_addr": walletAddr,
							"session_id":  sessionId,
						}).Infof(0, "successfully sent random number")
						return
					}
					time.Sleep(5 * time.Second)
				}
			}
		}(game, boxId, walletAddr, nc)
	
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
	}
}

func SendTestRandNum(nc *nats.Conn) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		sessionId := req.Header.Get("owl-session-id")
		walletAddr := req.URL.Query().Get("walletAddr")
		reqURL := req.URL
		urlPath := reqURL.Path
		logger.WithFields(logger.Fields{
			"caller":      "sendTestRandNum",
			"url_path":    urlPath,
			"game":        "roulette",
			"wallet_addr": walletAddr,
			"session_id":  sessionId,
		}).Infof(0, "sendRandNum called")

		go func(walletAddr string, nc *nats.Conn) {
			randSrc := rand.NewSource(time.Now().UnixNano())
			randNum := random(8, randSrc)

			wake := make(chan bool, 1)
			go wait(10 * time.Second, wake)

			for {
				select {
				case <-wake:
					topic := fmt.Sprintf("roulette.%s", walletAddr)
						nc.Publish(topic, []byte(randNum))
						logger.WithFields(logger.Fields{
							"caller":      "sendRandNum",
							"durationMs":  time.Since(start).Milliseconds(),
							"rand_num":    randNum,
							"game":        "roulette",
							"wallet_addr": walletAddr,
							"session_id":  sessionId,
						}).Infof(0, "successfully sent random number")
					return
				default:
				}
			}
		}(walletAddr, nc)
	
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
	}
}