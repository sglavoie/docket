package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var (
	taskDeleteForce bool
	taskDeleteJSON  bool
	taskDeleteQuiet bool
)

var taskDeleteCmd = &cobra.Command{
	Use:   "delete [task-id]",
	Short: "Permanently delete a task",
	Long: `Permanently delete a task and all its descendants.

This removes the task from the project entirely. This cannot be undone
(unless undo history is enabled). Dangling references in DependsOn and
RelatedTo fields of other tasks are cleaned up automatically.

If no task ID is provided, an interactive selector is shown.

Examples:
  tt task delete 42
  tt task delete 42 --force
  tt t delete abc123`,
	RunE: runTaskDelete,
}

func init() {
	taskCmd.AddCommand(taskDeleteCmd)
	taskDeleteCmd.Flags().BoolVarP(&taskDeleteForce, "force", "f", false, "Skip confirmation prompt")
	taskDeleteCmd.Flags().BoolVar(&taskDeleteJSON, "json", false, "output deleted task as JSON")
	taskDeleteCmd.Flags().BoolVar(&taskDeleteQuiet, "quiet", false, "suppress output on success")
}

func runTaskDelete(cmd *cobra.Command, args []string) error {
	out := MutateOutput{JSON: taskDeleteJSON, Quiet: taskDeleteQuiet}
	if err := out.CheckMutualExclusion(); err != nil {
		return err
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Select task: from args or interactive
	task, err := SelectTask(project, args, func(t *models.Task) bool {
		return true // all tasks are deletable
	}, "Select task to delete", "No tasks in this project.")
	if err != nil {
		return err
	}

	// Count descendants
	idsToDelete := collectTaskDescendantIDs(project.Tasks, map[string]bool{task.ID: true})
	count := len(idsToDelete)

	prompt := fmt.Sprintf("Permanently delete task #%d '%s'?", task.ShortID, task.Title)
	if count > 1 {
		prompt = fmt.Sprintf("Permanently delete task #%d '%s' and %s?",
			task.ShortID, task.Title, Pluralize(count-1, "descendant", "descendants"))
	}
	prompt += " [y/N] "

	confirmFn := ConfirmYN
	if count > 1 {
		confirmFn = ConfirmStrict
	}
	if !confirmFn(prompt, taskDeleteForce) {
		fmt.Println("Canceled")
		return nil
	}

	// Record undo snapshot
	if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
		if snapshotErr := storage.RecordUndoSnapshot(project, "Delete Task",
			fmt.Sprintf("#%d %s", task.ShortID, task.Title), *cfg.UndoHistoryDepth); snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot: %v\n", snapshotErr)
		}
	}

	// Clean up attachments for all tasks being deleted
	for _, t := range project.Tasks {
		if idsToDelete[t.ID] {
			if cleanErr := storage.CleanupTaskAttachments(project, t); cleanErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to clean up attachments for task #%d: %v\n", t.ShortID, cleanErr)
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

	// Save
	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Delete Task", UndoContext: fmt.Sprintf("#%d %s", task.ShortID, task.Title),
	}); err != nil {
		return err
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

// collectTaskDescendantIDs collects IDs of root tasks and all their descendants.
// This is a cmd-level version of the TUI's collectDescendantIDs.
func collectTaskDescendantIDs(tasks []*models.Task, rootIDs map[string]bool) map[string]bool {
	idsToDelete := make(map[string]bool)
	var collectIDs func(t *models.Task)
	collectIDs = func(t *models.Task) {
		idsToDelete[t.ID] = true
		for _, child := range t.Children {
			collectIDs(child)
		}
	}
	for _, t := range tasks {
		if rootIDs[t.ID] {
			collectIDs(t)
		}
	}
	return idsToDelete
}
