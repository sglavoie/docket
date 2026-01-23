package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui"
)

var taskFocusCmd = &cobra.Command{
	Use:   "focus [id]",
	Short: "Toggle the active/focus flag on a task",
	Long: `Toggle a task's active flag, adding or removing it from focus view.

If no ID is specified, opens an interactive selector.

Examples:
  tt task focus 5       # Toggle focus on task #5
  tt task focus         # Interactive selection`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskFocus,
}

var taskUnfocusCmd = &cobra.Command{
	Use:   "unfocus [id]",
	Short: "Remove a task from focus",
	Long: `Remove the active flag from a task, removing it from focus view.

Examples:
  tt task unfocus 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskUnfocus,
}

func init() {
	taskCmd.AddCommand(taskFocusCmd)
	taskCmd.AddCommand(taskUnfocusCmd)
}

func runTaskFocus(cmd *cobra.Command, args []string) error {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task

	if len(args) > 0 {
		task = FindTaskByIdentifier(project, args[0])
		if task == nil {
			return fmt.Errorf("task '%s' not found", args[0])
		}
	} else {
		// Interactive selection - show active (non-completed) tasks
		var candidates []*models.Task
		for _, t := range project.Tasks {
			if t.Status.IsActive() {
				candidates = append(candidates, t)
			}
		}

		if len(candidates) == 0 {
			return fmt.Errorf("no active tasks available")
		}

		items := TasksToSelectorItems(candidates)
		selected, err := tui.RunSelector("Select task to toggle focus:", items)
		if err != nil {
			return err
		}

		task, err = GetTaskFromSelector(selected)
		if err != nil {
			return err
		}
	}

	// Toggle active flag
	task.ToggleActive()

	// Save project
	if err := SaveProjectWithRules(project, cfg, configManager); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if task.Active {
		fmt.Printf("Added #%d to focus: %s\n", task.ShortID, task.Title)
	} else {
		fmt.Printf("Removed #%d from focus: %s\n", task.ShortID, task.Title)
	}

	return nil
}

func runTaskUnfocus(cmd *cobra.Command, args []string) error {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task

	if len(args) > 0 {
		task = FindTaskByIdentifier(project, args[0])
		if task == nil {
			return fmt.Errorf("task '%s' not found", args[0])
		}
	} else {
		// Interactive selection - show only active-flagged tasks
		var candidates []*models.Task
		for _, t := range project.Tasks {
			if t.Active && t.Status.IsActive() {
				candidates = append(candidates, t)
			}
		}

		if len(candidates) == 0 {
			return fmt.Errorf("no tasks in focus")
		}

		items := TasksToSelectorItems(candidates)
		selected, err := tui.RunSelector("Select task to remove from focus:", items)
		if err != nil {
			return err
		}

		task, err = GetTaskFromSelector(selected)
		if err != nil {
			return err
		}
	}

	if !task.Active {
		fmt.Printf("Task #%d is not in focus\n", task.ShortID)
		return nil
	}

	// Remove active flag
	task.Deactivate()

	// Save project
	if err := SaveProjectWithRules(project, cfg, configManager); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Removed #%d from focus: %s\n", task.ShortID, task.Title)
	return nil
}
