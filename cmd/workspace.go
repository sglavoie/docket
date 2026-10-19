package cmd

import (
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Inspect, back up, and relocate the workspace",
	Long: `Inspect, back up, verify, and relocate the workspace data directory.

New installations are local-first: tt stores data under ~/.tt unless an iCloud
Drive workspace already exists. iCloud (or any other location) is chosen
explicitly with 'tt workspace move'; there is no automatic migration. Use
'tt workspace backup' to save a verified copy outside the live workspace.

Run alone, 'tt workspace' prints the location, as 'tt workspace location' does.`,
	Args: noArgs,
	RunE: runWorkspaceLocation, // default to showing the current location
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceLocationCmd)
	workspaceCmd.AddCommand(workspaceMoveCmd)
	workspaceCmd.AddCommand(workspaceBackupCmd)
	workspaceCmd.AddCommand(workspaceVerifyBackupCmd)
}
