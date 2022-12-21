package controller

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/nightowlcasino/nightowl/logger"
	"go.uber.org/zap"
)


func Verbosity() httprouter.Handle {
	type Results struct {
		Level string `json:"verbosity"`
	}

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		log := zap.L()
		level := logger.GetLevel()
		log.Info("current logging level", zap.String("level", level))

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "{\"verbosity\": \"%s\"}", level)
	}
}

// SetVerbosity allows the user to remotely modify the verbosity of all log messages
// Expects a "v" parameter in the query string of a PUT request:
//
//     curl -X PUT http://host:port/api/v1/verbosity?v=debug
//
// options are:
//     debug
//     info
//     warn
//     error
//
func SetVerbosity() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		log := zap.L()

		level := r.URL.Query().Get("v")
		if level == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "{\"error\": \"missing or incorrect query parameter 'v='\"}")
			return
		}
		// If level was not set to any of the above it will default to info level
		logger.SetLevel(level)

		log.Info("updating logging level", zap.String("level", level))

		w.WriteHeader(http.StatusNoContent)
		fmt.Fprint(w, "")
	}
}