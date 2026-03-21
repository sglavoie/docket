package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
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
	RunE: runBoardDelete,
}

func init() {
	boardCmd.AddCommand(boardDeleteCmd)
	boardDeleteCmd.Flags().BoolVarP(&boardDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(boardDeleteCmd)
}

func runBoardDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("action ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project := FindActionTaskByID(cfg, args[0])
	if task == nil {
		return fmt.Errorf("action not found: %s. Use 'tt board list' to see available actions", args[0])
	}

	if !ConfirmYN(fmt.Sprintf("Delete action #%d '%s'? [y/N] ", task.ShortID, task.Title), boardDeleteForce) {
		if !out.Quiet {
			fmt.Println("Canceled")
		}
		return nil
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

	if err := SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return out.OutputTask(project.Name, "Deleted", task)
}
