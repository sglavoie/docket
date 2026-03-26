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

	// Collect task and all descendants for deletion (mirrors task_delete.go logic)
	idsToDelete := collectTaskDescendantIDs(project.Tasks, map[string]bool{task.ID: true})
	count := len(idsToDelete)

	// Clean up attachments for all tasks being deleted
	for _, t := range project.Tasks {
		if idsToDelete[t.ID] {
			if err := storage.CleanupTaskAttachments(project, t); err != nil {
				logging.Warn("failed to clean up attachments for task %s: %v", t.ID, err)
			}
		}
	}

	// Filter out deleted tasks and clear dangling references
	var remaining []*models.Task
	for _, t := range project.Tasks {
		if !idsToDelete[t.ID] {
			if t.ParentID != nil && idsToDelete[*t.ParentID] {
				t.ParentID = nil
			}
			// Clear dangling DependsOn references
			if len(t.DependsOn) > 0 {
				cleaned := t.DependsOn[:0]
				for _, depID := range t.DependsOn {
					if !idsToDelete[depID] {
						cleaned = append(cleaned, depID)
					}
				}
				t.DependsOn = cleaned
			}
			// Clear dangling RelatedTo references
			if len(t.RelatedTo) > 0 {
				cleaned := t.RelatedTo[:0]
				for _, relID := range t.RelatedTo {
					if !idsToDelete[relID] {
						cleaned = append(cleaned, relID)
					}
				}
				t.RelatedTo = cleaned
			}
			remaining = append(remaining, t)
		}
	}
	project.Tasks = remaining
	project.BuildTaskTree()

	if err := SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	// Clean cross-project references pointing to the deleted tasks from other projects.
	if err := cleanDeletedTaskCrossProjectRefs(cfg.Projects, project.ID, project.Slug, idsToDelete); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to clean cross-project references: %v\n", err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	msg := TaskMsg(project.Name, "Deleted", task)
	if count > 1 {
		msg += fmt.Sprintf(" and %s", Pluralize(count-1, "descendant", "descendants"))
	}
	fmt.Println(msg)
	return nil
}
