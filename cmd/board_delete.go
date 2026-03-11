package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var boardDeleteForce bool

var boardDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete an action permanently",
	Long: `Delete an action permanently from its project.

Examples:
  tt board delete 1
  tt board delete 1 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardDelete,
}

func init() {
	boardCmd.AddCommand(boardDeleteCmd)
	boardDeleteCmd.Flags().BoolVarP(&boardDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runBoardDelete(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project := FindActionTaskByID(cfg, args[0])
	if task == nil {
		return fmt.Errorf("action not found: %s. Use 'tt board list' to see available actions", args[0])
	}

	if !ConfirmYN(fmt.Sprintf("Delete action #%d '%s'? [y/N] ", task.ShortID, task.Title), boardDeleteForce) {
		fmt.Println("Canceled.")
		return nil
	}

	if backupErr := storage.BackupProject(project); backupErr != nil {
		return fmt.Errorf("failed to create backup: %w", backupErr)
	}

	if err := storage.CleanupTaskAttachments(project, task); err != nil {
		logging.Warn("failed to clean up attachments for task %s: %v", task.ID, err)
	}

	var remaining []*models.Task
	for _, t := range project.Tasks {
		if t.ID != task.ID {
			remaining = append(remaining, t)
		}
	}
	project.Tasks = remaining
	project.BuildTaskTree()

	if err := storage.SaveProject(project); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	fmt.Printf("Deleted action #%d: %s\n", task.ShortID, task.Title)
	return nil
}
