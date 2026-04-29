package cmd

import (
	"github.com/spf13/cobra"
)

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Manage epics",
	Long:  `Commands for managing epics and their scope baselines.`,
}

func init() {
	rootCmd.AddCommand(epicCmd)
}
