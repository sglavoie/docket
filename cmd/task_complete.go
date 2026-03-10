package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui"
)

var taskCompleteCmd = &cobra.Command{
	Use:     "complete [id...]",
	Aliases: []string{"done", "c"},
	Short:   "Mark task(s) as complete",
	Long: `Mark one or more tasks as done.

If no ID is specified, opens an interactive selector.

Examples:
  tt task complete 5
  tt task complete 1 2 3
  tt task done 3
  tt t c`,
	RunE: runTaskComplete,
}

var taskStartCmd = &cobra.Command{
	Use:   "start [id...]",
	Short: "Mark task(s) as in progress",
	Long: `Mark one or more tasks as in progress.

Examples:
  tt task start 5
  tt task start 1 2 3`,
	RunE: runTaskStart,
}

var taskBlockCmd = &cobra.Command{
	Use:   "block [id...]",
	Short: "Mark task(s) as blocked",
	Long: `Mark one or more tasks as blocked.

Examples:
  tt task block 5
  tt task block 1 2 3`,
	RunE: runTaskBlock,
}

var taskCancelCmd = &cobra.Command{
	Use:   "cancel [id...]",
	Short: "Cancel task(s)",
	Long: `Mark one or more tasks as canceled.

Examples:
  tt task cancel 5
  tt task cancel 1 2 3`,
	RunE: runTaskCancel,
}

var taskReopenCmd = &cobra.Command{
	Use:   "reopen [id...]",
	Short: "Reopen completed task(s)",
	Long: `Set one or more tasks back to todo status.

Examples:
  tt task reopen 5
  tt task reopen 1 2 3`,
	RunE: runTaskReopen,
}

var taskReviewCmd = &cobra.Command{
	Use:   "review [id...]",
	Short: "Send task(s) to review",
	Long: `Mark one or more tasks as in review (code review, QA, approval).

Examples:
  tt task review 5
  tt task review 1 2 3`,
	RunE: runTaskReview,
}

var taskStandbyCmd = &cobra.Command{
	Use:   "standby [id...]",
	Short: "Put task(s) on standby",
	Long: `Put one or more tasks on standby (paused, on hold).

Examples:
  tt task standby 5
  tt task standby 1 2 3`,
	RunE: runTaskStandby,
}

func init() {
	taskCmd.AddCommand(taskCompleteCmd)
	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskBlockCmd)
	taskCmd.AddCommand(taskCancelCmd)
	taskCmd.AddCommand(taskReopenCmd)
	taskCmd.AddCommand(taskReviewCmd)
	taskCmd.AddCommand(taskStandbyCmd)
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

func runTaskStandby(cmd *cobra.Command, args []string) error {
	return updateTaskStatus(args, "standby", func(t *models.Task) {
		t.Standby()
	})
}

func updateTaskStatus(args []string, action string, update func(*models.Task)) error {
	// Handle interactive selection if no ID provided
	var taskIDs []string
	if len(args) > 0 {
		taskIDs = args
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
			case "complete", "start", "block", "cancel", "review", "standby":
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
		taskIDs = []string{task.ID}
	}

	// Map action to past-tense verb for output
	actionVerbs := map[string]string{
		"complete": "Completed",
		"start":    "Started",
		"block":    "Blocked",
		"cancel":   "Canceled",
		"reopen":   "Reopened",
		"review":   "Set to review",
		"standby":  "Set to standby",
	}
	verb := actionVerbs[action]
	if verb == "" {
		verb = strings.ToUpper(action[:1]) + action[1:] + "d"
	}

	var errs []string
	succeeded := 0
	for _, taskID := range taskIDs {
		var generated int
		task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
			// Block completion if task has incomplete dependencies
			if action == "complete" {
				if incompleteDeps := project.GetIncompleteDependencies(task); len(incompleteDeps) > 0 {
					var msg strings.Builder
					fmt.Fprintf(&msg, "cannot complete task #%d: has %d incomplete dependencies:\n",
						task.ShortID, len(incompleteDeps))
					for _, dep := range incompleteDeps {
						fmt.Fprintf(&msg, "  - #%d %s (%s)\n", dep.ShortID, dep.Title, dep.Status)
					}
					return errors.New(msg.String())
				}
			}

			// Update status
			update(task)

			// Check for auto-unblocking
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
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		succeeded++
		fmt.Printf("%s %s #%d: %s\n", verb, task.Type.HeadingPrefix(), task.ShortID, task.Title)

		if generated > 0 {
			fmt.Printf("Created %d recurring task instance(s) in Inbox\n", generated)
		}
	}

	if len(errs) > 0 {
		if succeeded > 0 {
			fmt.Printf("\n%d succeeded, %d failed:\n", succeeded, len(errs))
		}
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
