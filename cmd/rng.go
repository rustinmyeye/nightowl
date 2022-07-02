package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// rngSvcCommand is responsible for listening to frontend requests for a games
// random number which it gets from the nightowl oracle pool
func rngSvcCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rng-svc",
		Short: "Run a server that listens for frontend requests for a games random number which it obtains from nightowls oracle pool.",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("setup rng svc")
		},
	}
}