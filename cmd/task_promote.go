package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui"
)

var taskPromoteCmd = &cobra.Command{
	Use:     "promote [id]",
	Aliases: []string{"up"},
	Short:   "Promote a task to a higher type",
	Long: `Promote a task to the next higher type in the hierarchy.

Type hierarchy: Task/Bug/Idea -> Story -> Epic

If no ID is specified, opens an interactive selector.

Examples:
  tt task promote 5    # Promote task #5 (e.g., Task -> Story)
  tt task up 3         # Using alias`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskPromote,
}

var taskDemoteCmd = &cobra.Command{
	Use:     "demote [id]",
	Aliases: []string{"down"},
	Short:   "Demote a task to a lower type",
	Long: `Demote a task to the next lower type in the hierarchy.

Type hierarchy: Epic -> Story -> Task

If no ID is specified, opens an interactive selector.

Examples:
  tt task demote 5     # Demote task #5 (e.g., Epic -> Story)
  tt task down 3       # Using alias`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskDemote,
}

var taskCycleCmd = &cobra.Command{
	Use:   "cycle [id]",
	Short: "Cycle a leaf task between Task, Bug, and Idea types",
	Long: `Cycle a leaf task's type among Task, Bug, and Idea.

By default cycles right: Task -> Bug -> Idea -> Task
Use --left to cycle in the opposite direction.

Only works on leaf types (Task, Bug, Idea).

Examples:
  tt task cycle 5          # Cycle right (Task -> Bug)
  tt task cycle 5 --left   # Cycle left (Task -> Idea)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskCycle,
}

var cycleLeft bool

func init() {
	taskCmd.AddCommand(taskPromoteCmd)
	taskCmd.AddCommand(taskDemoteCmd)
	taskCmd.AddCommand(taskCycleCmd)

	taskCycleCmd.Flags().BoolVarP(&cycleLeft, "left", "l", false, "Cycle left instead of right")
}

func runTaskPromote(cmd *cobra.Command, args []string) error {
	return updateTaskType(args, "promote", func(t *models.Task) error {
		return t.Promote()
	})
}

func runTaskDemote(cmd *cobra.Command, args []string) error {
	return updateTaskType(args, "demote", func(t *models.Task) error {
		return t.Demote()
	})
}

func runTaskCycle(cmd *cobra.Command, args []string) error {
	return updateTaskType(args, "cycle", func(t *models.Task) error {
		return t.CycleLeafType(cycleLeft)
	})
}

func updateTaskType(args []string, action string, update func(*models.Task) error) error {
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
		// Interactive selection - filter based on action
		var candidates []*models.Task
		for _, t := range project.Tasks {
			switch action {
			case "promote":
				if ok, _ := t.CanPromote(); ok {
					candidates = append(candidates, t)
				}
			case "demote":
				if ok, _ := t.CanDemote(); ok {
					candidates = append(candidates, t)
				}
			case "cycle":
				if ok, _ := t.CanCycleLeafType(cycleLeft); ok {
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

		task, err = GetTaskFromSelector(selected)
		if err != nil {
			return err
		}
	}

	// Record old type for output
	oldType := task.Type

	// Capture task states before changes for event-based workflow triggers
	prevStates := CaptureTaskStates(project.Tasks)

	// Update type
	if err := update(task); err != nil {
		return err
	}

	// Save with workflow rules evaluation
	if err := SaveProjectWithRulesAndChanges(project, cfg, configManager, prevStates); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("%s #%d: %s (%s -> %s)\n",
		task.Type.HeadingPrefix(),
		task.ShortID,
		task.Title,
		oldType,
		task.Type)

	return nil
}
