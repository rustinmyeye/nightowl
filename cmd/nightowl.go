package cmd

import (
	"github.com/nightowlcasino/nightowl/config"
	"github.com/spf13/cobra"
)

// NightOwl is the core component for all the backend services
func NightOwl() *cobra.Command {
	cmd := &cobra.Command{
		Use:   config.Application,
		Short: config.ApplicationFull,
		Long: `
Long Description`,
	}

	cmd.AddCommand(rngSvcCommand())
	cmd.AddCommand(payoutSvcCommand())

	return cmd
}
