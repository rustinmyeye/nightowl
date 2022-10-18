package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	http_no "github.com/nightowlcasino/nightowl/http"
	"github.com/spf13/viper"
)

const (
	// HeaderContentType is the Content-Type header key.
	HeaderContentType = "Content-Type"
	// ContentTypeJSON is the application/json MIME type.
	ContentTypeJSON = "application/json"
)

func NewServer(handler http.Handler) *http_no.Server {
	return http_no.NewServer(":"+strconv.Itoa(viper.Get("rng.port").(int)),
		handler,
		http_no.ReadTimeout(1*time.Minute),
		http_no.WriteTimeout(1*time.Minute),
		http_no.IdleTimeout(2*time.Minute))
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