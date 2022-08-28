package main

import (
	"github.com/nightowlcasino/nightowl/cmd"
	"github.com/nightowlcasino/nightowl/logger"
)

func main() {

	if err := cmd.Execute(); err != nil {
		logger.WithError(err).Infof(0, "failed to execute nightowl")
	}
}