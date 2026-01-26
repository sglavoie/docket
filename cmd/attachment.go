package cmd

import (
	"github.com/spf13/cobra"
)

var attachmentCmd = &cobra.Command{
	Use:   "attachment",
	Short: "Manage task attachments",
	Long: `Commands for opening and removing attachments on tasks.

Examples:
  tt attachment open 5 0      # Open first attachment on task 5
  tt attachment remove 5 0    # Remove first attachment from task 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to listing if no subcommand
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(attachmentCmd)
}
