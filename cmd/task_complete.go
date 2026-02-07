package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui"
)

var taskCompleteCmd = &cobra.Command{
	Use:     "complete [id]",
	Aliases: []string{"done", "c"},
	Short:   "Mark a task as complete",
	Long: `Mark a task as done.

If no ID is specified, opens an interactive selector.

Examples:
  tt task complete 5
  tt task done 3
  tt t c`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskComplete,
}

var taskStartCmd = &cobra.Command{
	Use:   "start [id]",
	Short: "Mark a task as in progress",
	Long: `Mark a task as in progress.

Examples:
  tt task start 5
  tt t start`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskStart,
}

var taskBlockCmd = &cobra.Command{
	Use:   "block [id]",
	Short: "Mark a task as blocked",
	Long: `Mark a task as blocked.

Examples:
  tt task block 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskBlock,
}

var taskCancelCmd = &cobra.Command{
	Use:   "cancel [id]",
	Short: "Cancel a task",
	Long: `Mark a task as canceled.

Examples:
  tt task cancel 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskCancel,
}

var taskReopenCmd = &cobra.Command{
	Use:   "reopen [id]",
	Short: "Reopen a completed task",
	Long: `Set a task back to todo status.

Examples:
  tt task reopen 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskReopen,
}

var taskReviewCmd = &cobra.Command{
	Use:   "review [id]",
	Short: "Send a task to review",
	Long: `Mark a task as in review (code review, QA, approval).

Examples:
  tt task review 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskReview,
}

func init() {
	taskCmd.AddCommand(taskCompleteCmd)
	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskBlockCmd)
	taskCmd.AddCommand(taskCancelCmd)
	taskCmd.AddCommand(taskReopenCmd)
	taskCmd.AddCommand(taskReviewCmd)
}

func runTaskComplete(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "complete", func(t *models.Task) {
		t.Complete()
	})
}

func runTaskStart(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "start", func(t *models.Task) {
		t.Start()
	})
}

func runTaskBlock(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "block", func(t *models.Task) {
		t.Block()
	})
}

func runTaskCancel(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "cancel", func(t *models.Task) {
		t.Cancel()
	})
}

func runTaskReopen(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "reopen", func(t *models.Task) {
		t.Reopen()
	})
}

func runTaskReview(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "review", func(t *models.Task) {
		t.Review()
	})
}

func updateTaskStatus(args []string, action string, update func(*models.Task)) error {
	// Handle interactive selection if no ID provided
	var taskID string
	if len(args) > 0 {
		taskID = args[0]
	} else {
		// Load project for interactive selection
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}

		// Filter candidates based on action
		var candidates []*models.Task
		for _, t := range project.Tasks {
			switch action {
			case "complete", "start", "block", "cancel", "review":
				if t.Status.IsActive() {
					candidates = append(candidates, t)
				}
			case "reopen":
				if t.Status.IsComplete() {
					candidates = append(candidates, t)
				}
			}
		}

		if len(candidates) == 0 {
			return fmt.Errorf("no tasks available for this action")
		}

		items := TasksToSelectorItems(candidates)
		prompt := fmt.Sprintf("Select task to %s:", action)
		selected, err := tui.RunSelector(prompt, items)
		if err != nil {
			return err
		}

		task, err := GetTaskFromSelector(selected)
		if err != nil {
			return err
		}
		taskID = task.ID
	}

	// Track if we generated recurring instances
	var generated int

	// Use WithTaskUpdateInProject since we need project context
	task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
		// Block completion if task has incomplete dependencies
		if action == "complete" {
			if incompleteDeps := project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
				msg := fmt.Sprintf("cannot complete task #%d: has %d incomplete dependencies:\n",
					task.ShortID, len(incompleteDeps))
				for _, dep := range incompleteDeps {
					msg += fmt.Sprintf("  - #%d %s (%s)\n", dep.ShortID, dep.Title, dep.Status)
				}
				return errors.New(msg)
			}
		}

		// Update status
		update(task)

		// Check for auto-unblocking (when task is completed/canceled, its dependents may unblock)
		if task.Status == models.StatusDone || task.Status == models.StatusCanceled {
			project.CheckAndAutoUnblock()
		}

		// Generate recurring task instances for completed recurring tasks
		if action == "complete" {
			generated = project.GenerateRecurringInstances()
		}

		return nil
	})

	if err != nil {
		return err
	}

	fmt.Printf("%s #%d: %s -> %s\n",
		task.Type.HeadingPrefix(),
		task.ShortID,
		task.Title,
		task.Status)

	// Notify user about generated recurring instances
	if generated > 0 {
		fmt.Printf("Created %d recurring task instance(s) in Inbox\n", generated)
	}

	return nil
}
