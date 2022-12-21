package controller

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/go-redis/redis/v9"
	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/nats.go"
	"github.com/nightowlcasino/nightowl/buildinfo"
	"go.uber.org/zap"
)

type Router struct {
	http.Handler

	ready bool
}

func (r *Router) Ready() {
	r.ready = true
}

func NewRouter(nats *nats.Conn, rdb *redis.Client) *Router {
	h := httprouter.New()
	h.RedirectTrailingSlash = false
	h.RedirectFixedPath = false

	r := &Router{
		Handler: h,
	}

	// this limiter is set to only handle 1req/10secs
	limitr := tollbooth.NewLimiter(.1, nil).
		SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}).
		SetMessage("Please slow down your requests :)").
		SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
			log := zap.L()
			reqURL := r.URL
			urlPath := reqURL.Path
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)

			// This will only be defined when site is accessed via non-anonymous proxy
    		// and takes precedence over RemoteAddr
    		// Header.Get is case-insensitive
    		forward := r.Header.Get("X-Forwarded-For")
			if forward != "" {
				ip = forward
			}

			log.Debug("an attempt to spam notifications was made",
				zap.String("url_path", urlPath),
				zap.String("ip_addr", ip),
			)
		})

	h.GET("/info", func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		json.NewEncoder(w).Encode(buildinfo.Info)
	})

	h.GET("/api/v1/random-number/:game", SendRandNum(nats))
	h.OPTIONS("/api/v1/random-number/:game", opts())

	h.GET("/api/v1/test/random-number/roulette", SendTestRandNum(nats))
	h.OPTIONS("/api/v1/test/random-number/roulette", opts())

	h.GET("/api/v1/notifs/:walletAddr", LimitHandler(SendNotifs(nats, rdb), limitr))
	h.OPTIONS("/api/v1/notifs/:walletAddr", opts())

	r.ready = true

	return r
}

func LimitHandler(handler httprouter.Handle, lmt *limiter.Limiter) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		httpError := tollbooth.LimitByRequest(lmt, w, r)
		if httpError != nil {
			lmt.ExecOnLimitReached(w, r)
			w.Header().Add("Content-Type", lmt.GetMessageContentType())
			w.WriteHeader(httpError.StatusCode)
			w.Write([]byte(httpError.Message))
			return
		}

		handler(w, r, ps)
	}
}