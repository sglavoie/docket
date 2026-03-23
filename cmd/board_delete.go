package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var boardDeleteForce bool

var boardDeleteCmd = &cobra.Command{
	Use:     "delete [id]",
	Aliases: []string{"rm"},
	Short:   "Delete an action permanently",
	Long: `Delete an action permanently from its project.

If no ID is provided, an interactive selector is shown.

Examples:
  tt board delete 1
  tt board delete 1 --force
  tt board delete`,
	RunE: runBoardDelete,
}

func init() {
	boardCmd.AddCommand(boardDeleteCmd)
	boardDeleteCmd.Flags().BoolVarP(&boardDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(boardDeleteCmd)
}

func runBoardDelete(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project, err := SelectActionTask(cfg, args, "Select action to delete:", "No actions found. Use 'tt board add' to create one.")
	if err != nil {
		return err
	}

	if !ConfirmYN(fmt.Sprintf("Delete action #%d '%s'? [y/N]: ", task.ShortID, task.Title), boardDeleteForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
		if snapshotErr := storage.RecordUndoSnapshot(project, "Delete Action",
			fmt.Sprintf("#%d %s", task.ShortID, task.Title), *cfg.UndoHistoryDepth); snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot: %v\n", snapshotErr)
		}
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
