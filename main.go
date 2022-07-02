package main

import (
	"fmt"

	"github.com/nightowlcasino/nightowl/cmd"
)

func main() {

	if err := cmd.NightOwl().Execute(); err != nil {
		fmt.Println("failed to execute Nightowl")
	}
}