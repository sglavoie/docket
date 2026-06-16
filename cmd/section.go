package cmd

import (
	"github.com/spf13/cobra"
)

var sectionCmd = &cobra.Command{
	Use:     "section",
	Aliases: []string{"sec"},
	Short:   "Manage task sections",
	Long:    `List built-in GTD task sections. Flexible grouping now uses tags and pinned tag tabs.`,
	RunE:    runSectionList, // Default to listing sections
}

func init() {
	rootCmd.AddCommand(sectionCmd)
}
