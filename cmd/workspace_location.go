package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/logging"
)

var workspaceLocationCmd = &cobra.Command{
	Use:   "location",
	Short: "Show the current workspace data directory",
	Long: `Show where the current workspace stores its data, the workspace catalog
(.docket/workspace.json) inside it, whether that location is local or synced, the
machine-local active-workspace pointer (if one has been set by 'docket workspace
move'), and where logs, migration backups, project exports, and merge
conflicts live.`,
	Example: `  docket workspace location
  docket --data-dir ~/Documents/docket workspace location`,
	Args:              noArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE:              runWorkspaceLocation,
}

func runWorkspaceLocation(_ *cobra.Command, _ []string) error {
	if err := checkDataDirOverride(); err != nil {
		return err
	}
	manager := config.NewManager(resolveConfigPath())
	configPath := manager.GetConfigPath()
	workspaceDir := config.WorkspaceRootFromConfigPath(configPath)

	fmt.Printf("Workspace data directory: %s\n", config.DisplayPath(workspaceDir))
	// Every path below is where the workspace would live; say when none does yet
	// rather than list files that are not there.
	if exists, err := workspaceExists(configPath); err == nil && !exists {
		fmt.Println("                          (no workspace here yet; run 'docket' to create one)")
	}
	fmt.Printf("Catalog:                  %s\n", config.DisplayPath(config.WorkspaceCatalogPath(workspaceDir)))
	fmt.Printf("Storage:                  %s\n", config.StorageLocationLabel(workspaceDir))

	// An explicit --data-dir or DOCKET_DATA_DIR wins over the pointer, so the line
	// says so rather than imply the pointer or the default chose this workspace.
	pointer := "(none — using default resolution)"
	if ptr, ok := config.ReadActiveWorkspacePointer(); ok {
		pointer = config.DisplayPath(ptr)
	} else if GetDataDir() != "" {
		pointer = "(none)"
	}
	if GetDataDir() != "" {
		pointer += fmt.Sprintf(" (overridden by %s)", dataDirOverrideSource())
	}
	fmt.Printf("Active workspace pointer: %s\n", pointer)

	if logsDir, err := logging.Dir(); err == nil {
		fmt.Printf("Logs:                     %s (machine-local)\n", config.DisplayPath(logsDir))
	} else {
		fmt.Println("Logs:                     (unavailable — no home directory)")
	}
	fmt.Printf("Backups:                  %s\n", config.DisplayPath(config.PreScopeV7BackupsDir(workspaceDir)))
	fmt.Printf("Exports:                  %s\n", config.DisplayPath(manager.ExportsDir()))
	fmt.Printf("Conflicts:                %s\n", config.DisplayPath(config.ConflictsDir(workspaceDir)))

	fmt.Println()
	fmt.Println("Relocate with: docket workspace move <destination>")
	return nil
}
