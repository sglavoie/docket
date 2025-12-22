package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui"
	"github.com/spf13/cobra"
)

var taskMoveCmd = &cobra.Command{
	Use:   "move",
	Short: "Move tasks in the hierarchy",
	Long: `Commands for moving tasks up/down among siblings or indenting/outdenting in the hierarchy.

Examples:
  tt task move up 5       # Move task #5 up among its siblings
  tt task move down 5     # Move task #5 down among its siblings
  tt task move indent 5   # Nest task #5 under the task above it
  tt task move outdent 5  # Move task #5 to its parent's level`,
}

var taskMoveUpCmd = &cobra.Command{
	Use:   "up [id]",
	Short: "Move a task up among its siblings",
	Long: `Move a task up among its siblings (tasks with the same parent).

If no ID is specified, opens an interactive selector.

Examples:
  tt task move up 5    # Move task #5 up`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskMoveUp,
}

var taskMoveDownCmd = &cobra.Command{
	Use:   "down [id]",
	Short: "Move a task down among its siblings",
	Long: `Move a task down among its siblings (tasks with the same parent).

If no ID is specified, opens an interactive selector.

Examples:
  tt task move down 5    # Move task #5 down`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskMoveDown,
}

var taskIndentCmd = &cobra.Command{
	Use:   "indent [id]",
	Short: "Nest a task under the task above it",
	Long: `Indent a task to become a child of the task visually above it.

The target parent must be able to have children (Epic or Story).
The task's type must be valid under the target parent.

If no ID is specified, opens an interactive selector.

Examples:
  tt task move indent 5    # Nest task #5 under the task above it`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskIndent,
}

var taskOutdentCmd = &cobra.Command{
	Use:   "outdent [id]",
	Short: "Move a task to its parent's level",
	Long: `Outdent a task to become a sibling of its current parent.

The task must have a parent (not be a root task).
The task's type must be valid at the new level.

If no ID is specified, opens an interactive selector.

Examples:
  tt task move outdent 5    # Move task #5 to its parent's level`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskOutdent,
}

func init() {
	taskCmd.AddCommand(taskMoveCmd)
	taskMoveCmd.AddCommand(taskMoveUpCmd)
	taskMoveCmd.AddCommand(taskMoveDownCmd)
	taskMoveCmd.AddCommand(taskIndentCmd)
	taskMoveCmd.AddCommand(taskOutdentCmd)
}

func runTaskMoveUp(cmd *cobra.Command, args []string) error {
	project, _, _, err := getCurrentProject()
	if err != nil {
		return err
	}

	task, err := selectTaskForMove(project, args, "up")
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	// Check if can move up
	siblings := project.GetSiblings(task)
	var siblingIdx int = -1
	for i, s := range siblings {
		if s.ID == task.ID {
			siblingIdx = i
			break
		}
	}
	if siblingIdx <= 0 {
		fmt.Println("Task is already at the top of its siblings")
		return nil
	}

	// Check completion boundary for root tasks
	siblingAbove := siblings[siblingIdx-1]
	if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
		fmt.Println("Cannot move completed task above uncompleted task")
		return nil
	}

	if err := project.MoveTaskUp(task); err != nil {
		return fmt.Errorf("failed to move task: %w", err)
	}

	if err := saveProject(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Moved #%d: %s up\n", task.ShortID, task.Title)
	return nil
}

func runTaskMoveDown(cmd *cobra.Command, args []string) error {
	project, _, _, err := getCurrentProject()
	if err != nil {
		return err
	}

	task, err := selectTaskForMove(project, args, "down")
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	// Check if can move down
	siblings := project.GetSiblings(task)
	var siblingIdx int = -1
	for i, s := range siblings {
		if s.ID == task.ID {
			siblingIdx = i
			break
		}
	}
	if siblingIdx == -1 || siblingIdx >= len(siblings)-1 {
		fmt.Println("Task is already at the bottom of its siblings")
		return nil
	}

	// Check completion boundary for root tasks
	siblingBelow := siblings[siblingIdx+1]
	if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
		fmt.Println("Cannot move uncompleted task below completed task")
		return nil
	}

	if err := project.MoveTaskDown(task); err != nil {
		return fmt.Errorf("failed to move task: %w", err)
	}

	if err := saveProject(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Moved #%d: %s down\n", task.ShortID, task.Title)
	return nil
}

func runTaskIndent(cmd *cobra.Command, args []string) error {
	project, _, _, err := getCurrentProject()
	if err != nil {
		return err
	}

	task, err := selectTaskForMove(project, args, "indent")
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	// Get target parent
	targetParent := project.GetTaskAbove(task)
	if targetParent == nil {
		fmt.Println("No task above to nest under")
		return nil
	}

	// Check if can indent
	if ok, reason := task.CanIndent(targetParent); !ok {
		fmt.Printf("Cannot indent: %s\n", reason)
		return nil
	}

	if err := project.IndentTask(task); err != nil {
		return fmt.Errorf("failed to indent task: %w", err)
	}

	if err := saveProject(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Indented #%d: %s (now under #%d: %s)\n",
		task.ShortID, task.Title,
		targetParent.ShortID, targetParent.Title)
	return nil
}

func runTaskOutdent(cmd *cobra.Command, args []string) error {
	project, _, _, err := getCurrentProject()
	if err != nil {
		return err
	}

	task, err := selectTaskForMove(project, args, "outdent")
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	// Check if can outdent
	if ok, reason := task.CanOutdent(); !ok {
		fmt.Printf("Cannot outdent: %s\n", reason)
		return nil
	}

	oldParent := task.Parent

	if err := project.OutdentTask(task); err != nil {
		return fmt.Errorf("failed to outdent task: %w", err)
	}

	if err := saveProject(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if task.ParentID == nil {
		fmt.Printf("Outdented #%d: %s (now at root level, was under #%d: %s)\n",
			task.ShortID, task.Title,
			oldParent.ShortID, oldParent.Title)
	} else {
		newParent := project.GetTaskByID(*task.ParentID)
		fmt.Printf("Outdented #%d: %s (now under #%d: %s)\n",
			task.ShortID, task.Title,
			newParent.ShortID, newParent.Title)
	}
	return nil
}

// selectTaskForMove finds a task by ID or presents an interactive selector.
func selectTaskForMove(project *models.Project, args []string, action string) (*models.Task, error) {
	if len(args) > 0 {
		task := findTask(project, args[0])
		if task == nil {
			return nil, fmt.Errorf("task '%s' not found", args[0])
		}
		return task, nil
	}

	// Interactive selection - filter based on action
	var candidates []*models.Task
	for _, t := range project.Tasks {
		switch action {
		case "up":
			siblings := project.GetSiblings(t)
			for i, s := range siblings {
				if s.ID == t.ID && i > 0 {
					candidates = append(candidates, t)
					break
				}
			}
		case "down":
			siblings := project.GetSiblings(t)
			for i, s := range siblings {
				if s.ID == t.ID && i < len(siblings)-1 {
					candidates = append(candidates, t)
					break
				}
			}
		case "indent":
			targetParent := project.GetTaskAbove(t)
			if targetParent != nil {
				if ok, _ := t.CanIndent(targetParent); ok {
					candidates = append(candidates, t)
				}
			}
		case "outdent":
			if ok, _ := t.CanOutdent(); ok {
				candidates = append(candidates, t)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no tasks available for this action")
	}

	items := make([]tui.SelectorItem, len(candidates))
	for i, t := range candidates {
		items[i] = tui.SelectorItem{
			ID:   t.ID,
			Data: t,
			Columns: []string{
				fmt.Sprintf("#%d", t.ShortID),
				t.Type.Icon(),
				t.Status.Icon(),
				t.Title,
			},
		}
	}

	prompt := fmt.Sprintf("Select task to move %s:", action)
	selected, err := tui.RunSelector(prompt, items)
	if err != nil {
		return nil, err
	}

	return selected.Data.(*models.Task), nil
}
