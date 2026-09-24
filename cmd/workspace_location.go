package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

var workspaceLocationCmd = &cobra.Command{
	Use:   "location",
	Short: "Show the current workspace data directory",
	Long: `Show where the current workspace stores its data, which config file backs
it, whether that location is local or synced, the machine-local
active-workspace pointer (if one has been set by 'tt workspace move'), and
where logs, migration backups, project exports, and merge conflicts live.`,
	Args: cobra.NoArgs,
	RunE: runWorkspaceLocation,
}

func runWorkspaceLocation(_ *cobra.Command, _ []string) error {
	manager := config.NewManager(resolveConfigPath())
	configPath := manager.GetConfigPath()
	workspaceDir := config.WorkspaceRootFromConfigPath(configPath)

	fmt.Printf("Workspace data directory: %s\n", config.DisplayPath(workspaceDir))
	fmt.Printf("Config file:              %s\n", config.DisplayPath(configPath))
	fmt.Printf("Storage:                  %s\n", config.StorageLocationLabel(workspaceDir))

	if ptr, ok := config.ReadActiveWorkspacePointer(); ok {
		fmt.Printf("Active workspace pointer: %s\n", config.DisplayPath(ptr))
	} else {
		fmt.Println("Active workspace pointer: (none — using default resolution)")
	}

	if logsDir, err := logging.Dir(); err == nil {
		fmt.Printf("Logs:                     %s (machine-local)\n", config.DisplayPath(logsDir))
	} else {
		fmt.Println("Logs:                     (unavailable — no home directory)")
	}
	fmt.Printf("Backups:                  %s\n", config.DisplayPath(config.PreScopeV7BackupsDir(workspaceDir)))
	fmt.Printf("Exports:                  %s\n", config.DisplayPath(manager.ExportsDir()))
	fmt.Printf("Conflicts:                %s\n", config.DisplayPath(config.ConflictsDir(workspaceDir)))

	fmt.Println()
	fmt.Println("Relocate with: tt workspace move <destination>")
	return nil
}
