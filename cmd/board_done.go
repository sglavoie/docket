package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var boardDoneCmd = &cobra.Command{
	Use:   "done [id]",
	Short: "Mark an action as done",
	Long: `Mark an action item as done.

If no ID is provided, an interactive selector is shown.

Examples:
  tt board done 1
  tt board done 3
  tt board done`,
	RunE: runBoardDone,
}

func init() {
	boardCmd.AddCommand(boardDoneCmd)
	addOutputFlags(boardDoneCmd)
}

func runBoardDone(cmd *cobra.Command, args []string) error {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	task, project, err := SelectActionTask(cfg, args, "Select action to mark done:", "No actions found. Use 'tt board add' to create one.")
	if err != nil {
		return err
	}

	if msg := task.CanTransitionTo(models.StatusDone); msg != "" {
		return fmt.Errorf("cannot mark done: %s", msg)
	}

	// Block completion if task has incomplete dependencies
	if project.HasIncompleteDependencies(task) {
		incompleteDeps := project.GetIncompleteDependencies(task)
		var msg strings.Builder
		fmt.Fprintf(&msg, "cannot complete task #%d: has incomplete dependencies:\n", task.ShortID)
		for _, dep := range incompleteDeps {
			fmt.Fprintf(&msg, "  - #%d %s (%s)\n", dep.ShortID, dep.Title, dep.Status)
		}
		crossProjectCount := countCrossProjectDeps(task)
		if crossProjectCount > 0 {
			fmt.Fprintf(&msg, "  - %d cross-project dependency/dependencies (not resolvable locally)\n", crossProjectCount)
		}
		return errors.New(msg.String())
	}

	// Block completion if task has incomplete children
	var incompleteChildren []*models.Task
	for _, child := range task.Children {
		if child.Status != models.StatusDone && child.Status != models.StatusCanceled {
			incompleteChildren = append(incompleteChildren, child)
		}
	}
	if len(incompleteChildren) > 0 {
		var msg strings.Builder
		fmt.Fprintf(&msg, "cannot complete task #%d: has %d incomplete children:\n",
			task.ShortID, len(incompleteChildren))
		for _, child := range incompleteChildren {
			fmt.Fprintf(&msg, "  - #%d %s (%s)\n", child.ShortID, child.Title, child.Status)
		}
		return errors.New(msg.String())
	}

	prevStates := CaptureTaskStates(project.Tasks)

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusDone)
	})

	project.CheckAndAutoUnblock()

	// Generate recurring task instances for completed recurring tasks
	generated := project.GenerateRecurringInstances()

	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: mgr,
		PrevStates: prevStates,
		UndoAction: "Board Done", UndoContext: fmt.Sprintf("#%d %s", task.ShortID, task.Title),
	}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if err := out.OutputTask(project.Name, "Done", task); err != nil {
		return err
	}
	if !out.Quiet && !out.JSON && generated > 0 {
		fmt.Printf("Created %s in Inbox\n", Pluralize(generated, "recurring task instance", "recurring task instances"))
	}
	return nil
}
