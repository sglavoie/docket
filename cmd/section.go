package cmd

import (
	"github.com/spf13/cobra"
)

var sectionCmd = &cobra.Command{
	Use:     "section",
	Aliases: []string{"sec"},
	Short:   "Manage task sections",
	Long:    `Commands for creating, listing, and managing task sections.`,
	RunE:    runSectionList, // Default to listing sections
}

func init() {
	rootCmd.AddCommand(sectionCmd)
}
