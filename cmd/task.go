package cmd

import (
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:     "task",
	Aliases: []string{"t"},
	Short:   "Manage tasks",
	Long:    `Commands for creating, listing, and managing tasks within the current project.`,
	RunE:    runTaskList, // Default to listing tasks
}

func init() {
	rootCmd.AddCommand(taskCmd)
}
