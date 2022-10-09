package main

import (
	"github.com/nightowlcasino/nightowl/cmd"
	"go.uber.org/zap"
)

var (
	log *zap.Logger
)

func main() {
	
	if err := cmd.Execute(); err != nil {
		log = zap.L()
		log.Error("failed to execute nightowl", zap.Error(err))
	}
}