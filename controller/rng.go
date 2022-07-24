package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/services/rng"
	log "github.com/sirupsen/logrus"
)

const (
	// HeaderContentType is the Content-Type header key.
	HeaderContentType = "Content-Type"
	// ContentTypeJSON is the application/json MIME type.
	ContentTypeJSON = "application/json"
)

func SendRandNum(logger *log.Entry, nc *nats.Conn) httprouter.Handle {
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
		logger.WithFields(log.Fields{"caller": "sendRandNum", "url_path": urlPath, "box_id": boxId, "game": game, "wallet_addr": walletAddr, "session_id": sessionId}).Info("sendRandNum called")

		go func(game, boxId, walletAddr string, logger *log.Entry, nc *nats.Conn) {
			timeout := time.NewTicker(120 * time.Second)
			for {
				select {
				case <-timeout.C:
					logger.WithFields(log.Fields{"caller": "sendRandNum", "durationMs": time.Since(start).Milliseconds(), "box_id": boxId, "game": game, "wallet_addr": walletAddr, "session_id": sessionId}).Error("timeout - random number not found")
					return
				default:
					if randNum, ok := rng.GetRandHashMap().Get(boxId); ok {
						topic := fmt.Sprintf("%s.%s", game, walletAddr)
						nc.Publish(topic, []byte(randNum))
						logger.WithFields(log.Fields{"caller": "sendRandNum", "durationMs": time.Since(start).Milliseconds(), "rand_num": randNum, "box_id": boxId, "game": game, "wallet_addr": walletAddr, "session_id": sessionId}).Info("successfully sent random number")
						return
					}
					time.Sleep(5 * time.Second)
				}
			}
		}(game, boxId, walletAddr, logger, nc)
	
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "")
	}
}