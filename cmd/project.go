package cmd

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"p"},
	Short:   "Manage projects",
	Long:    `Commands for creating, listing, and managing projects.`,
	RunE:    runProjectList, // Default to listing projects
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
