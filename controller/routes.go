package controller

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/services/rng"
	log "github.com/sirupsen/logrus"
)

type Router struct {
	http.Handler

	rng   *rng.Service
	ready bool
}

func (r *Router) Ready() {
	r.ready = true
}

func NewRouter(logger *log.Entry, nats *nats.Conn) *Router {
	h := httprouter.New()
	h.RedirectTrailingSlash = false
	h.RedirectFixedPath = false

	r := &Router{
		Handler: h,
	}

	h.GET("/api/v1/random-number/:game", SendRandNum(logger, nats))
	h.OPTIONS("/api/v1/random-number/{game}", opts())

	r.ready = true

	return r
}