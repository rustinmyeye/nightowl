package controller

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/services/rng"
)

type Router struct {
	http.Handler

	rng   *rng.Service
	ready bool
}

func (r *Router) Ready() {
	r.ready = true
}

func NewRouter(nats *nats.Conn) *Router {
	h := httprouter.New()
	h.RedirectTrailingSlash = false
	h.RedirectFixedPath = false

	r := &Router{
		Handler: h,
	}

	h.GET("/api/v1/random-number/:game", SendRandNum(nats))
	h.OPTIONS("/api/v1/random-number/:game", opts())

	h.GET("/api/v1/test/random-number/roulette", SendTestRandNum(nats))
	h.OPTIONS("/api/v1/test/random-number/roulette", opts())

	r.ready = true

	return r
}