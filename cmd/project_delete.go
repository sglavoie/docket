package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectDeleteCmd = &cobra.Command{
	Use:   "delete [name|id]",
	Short: "Delete a project (moves to trash)",
	Long: `Delete a project by moving it to the trash folder.

The project directory is moved to ~/.tt-data/.trash/ and can be restored later.
Use 'tt project restore <slug>' to restore a deleted project.

Examples:
  tt project delete "Old Project"
  tt p delete my-project`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectDelete,
}

func init() {
	projectCmd.AddCommand(projectDeleteCmd)
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	project := cfg.GetProject(args[0])
	if project == nil {
		return tterrors.NewProjectNotFoundError(args[0])
	}

	// Purge any existing trash entry with same slug
	if err := storage.PurgeFromTrash(project.Slug, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to purge old trash entry: %w", err)
	}

	// Backup config before making changes
	if err := configManager.BackupConfig(); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	// Move project directory to trash
	if err := storage.MoveProjectToTrash(project, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to move project to trash: %w", err)
	}

	// Remove project from config
	cfg.RemoveProject(project.ID)

	// Save config
	if err := configManager.Save(cfg); err != nil {
		// Try to restore from trash on failure (best effort)
		_ = storage.RestoreProjectFromTrash(project.Slug, cfg.DataDir)
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Deleted project '%s'\n", project.Name)
	fmt.Printf("To restore: tt project restore %s\n", project.Slug)
	return nil
}
