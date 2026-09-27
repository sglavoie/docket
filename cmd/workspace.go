package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage the workspace data directory location",
	Long: `Inspect and relocate the workspace data directory.

New installations are local-first: tt stores data under ~/.tt unless an iCloud
Drive workspace already exists. iCloud (or any other location) is chosen
explicitly with 'tt workspace move'; there is no automatic migration.

Run alone, 'tt workspace' prints the location, as 'tt workspace location' does.`,
	Args: noArgs,
	RunE: runWorkspaceLocation, // default to showing the current location
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceLocationCmd)
	workspaceCmd.AddCommand(workspaceMoveCmd)
}
