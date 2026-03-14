package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectDeleteForce bool

var projectDeleteCmd = &cobra.Command{
	Use:   "delete [name|id]",
	Short: "Delete a project",
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
	projectDeleteCmd.Flags().BoolVarP(&projectDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	project, cfg, configManager, err := LoadConfigAndProject(args[0])
	if err != nil {
		return err
	}

	if project.Sandbox {
		return fmt.Errorf("cannot delete the sandbox project; use 'tt sandbox reset' to clear its contents instead")
	}

	if !ConfirmYN(fmt.Sprintf("Delete project '%s'? It will be moved to trash. [y/N] ", project.Name), projectDeleteForce) {
		fmt.Println("Canceled")
		return nil
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
		// Try to restore from trash on failure
		if restoreErr := storage.RestoreProjectFromTrash(project.Slug, cfg.DataDir); restoreErr != nil {
			return fmt.Errorf("config save failed and restore from trash also failed: %w (restore error: %w)", err, restoreErr)
		}
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Deleted project '%s'\n", project.Name)
	fmt.Printf("To restore: tt project restore %s\n", project.Slug)
	return nil
}
