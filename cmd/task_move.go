package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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

	addOutputFlags(taskMoveUpCmd)
	addOutputFlags(taskMoveDownCmd)
	addOutputFlags(taskIndentCmd)
	addOutputFlags(taskOutdentCmd)
}

// runTaskMoveAction is the generic skeleton for all move subcommands:
// Load → Select → validate → move → Save → print.
func runTaskMoveAction(
	args []string,
	action string,
	out MutateOutput,
	validate func(*models.Project, *models.Task) error,
	move func(*models.Project, *models.Task) error,
	describe func(*models.Project, *models.Task) string,
) error {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, err := selectTaskForMove(project, args, action)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	if err := validate(project, task); err != nil {
		return err
	}

	if err := move(project, task); err != nil {
		return fmt.Errorf("failed to %s task: %w", action, err)
	}

	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Move Task", UndoContext: fmt.Sprintf("%s #%d", action, task.ShortID),
	}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		fmt.Println(describe(project, task))
	}
	return nil
}

func runTaskMoveUp(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	return runTaskMoveAction(args, "up", out,
		func(p *models.Project, t *models.Task) error {
			siblings := p.GetSiblings(t)
			idx := p.FindSiblingIndex(t)
			if idx <= 0 {
				return fmt.Errorf("task is already at the top of its siblings")
			}
			if t.IsRoot() && t.IsComplete() && !siblings[idx-1].IsComplete() {
				return fmt.Errorf("cannot move completed task above uncompleted task")
			}
			return nil
		},
		func(p *models.Project, t *models.Task) error { return p.MoveTaskUp(t) },
		func(p *models.Project, t *models.Task) string {
			return fmt.Sprintf("Moved #%d: %s up", t.ShortID, t.Title)
		},
	)
}

func runTaskMoveDown(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	return runTaskMoveAction(args, "down", out,
		func(p *models.Project, t *models.Task) error {
			siblings := p.GetSiblings(t)
			idx := p.FindSiblingIndex(t)
			if idx == -1 || idx >= len(siblings)-1 {
				return fmt.Errorf("task is already at the bottom of its siblings")
			}
			if t.IsRoot() && !t.IsComplete() && siblings[idx+1].IsComplete() {
				return fmt.Errorf("cannot move uncompleted task below completed task")
			}
			return nil
		},
		func(p *models.Project, t *models.Task) error { return p.MoveTaskDown(t) },
		func(p *models.Project, t *models.Task) string {
			return fmt.Sprintf("Moved #%d: %s down", t.ShortID, t.Title)
		},
	)
}

func runTaskIndent(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	return runTaskMoveAction(args, "indent", out,
		func(p *models.Project, t *models.Task) error {
			targetParent := p.GetTaskAbove(t)
			if targetParent == nil {
				return fmt.Errorf("no task above to nest under")
			}
			if ok, reason := t.CanIndent(targetParent); !ok {
				return fmt.Errorf("cannot indent: %s", reason)
			}
			return nil
		},
		func(p *models.Project, t *models.Task) error { return p.IndentTask(t) },
		func(p *models.Project, t *models.Task) string {
			if t.Parent != nil {
				return fmt.Sprintf("Indented #%d: %s (now under #%d: %s)",
					t.ShortID, t.Title, t.Parent.ShortID, t.Parent.Title)
			}
			return fmt.Sprintf("Indented #%d: %s", t.ShortID, t.Title)
		},
	)
}

func runTaskOutdent(cmd *cobra.Command, args []string) error {
	// Capture parent before the move since OutdentTask changes it
	var oldParentShortID int
	var oldParentTitle string

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	return runTaskMoveAction(args, "outdent", out,
		func(p *models.Project, t *models.Task) error {
			if ok, reason := t.CanOutdent(); !ok {
				return fmt.Errorf("cannot outdent: %s", reason)
			}
			if t.Parent != nil {
				oldParentShortID = t.Parent.ShortID
				oldParentTitle = t.Parent.Title
			}
			return nil
		},
		func(p *models.Project, t *models.Task) error { return p.OutdentTask(t) },
		func(p *models.Project, t *models.Task) string {
			if t.ParentID == nil {
				return fmt.Sprintf("Outdented #%d: %s (now at root level, was under #%d: %s)",
					t.ShortID, t.Title, oldParentShortID, oldParentTitle)
			}
			newParent := p.GetTaskByID(*t.ParentID)
			if newParent == nil {
				return fmt.Sprintf("Outdented #%d: %s", t.ShortID, t.Title)
			}
			return fmt.Sprintf("Outdented #%d: %s (now under #%d: %s)",
				t.ShortID, t.Title, newParent.ShortID, newParent.Title)
		},
	)
}

// selectTaskForMove finds a task by ID or presents an interactive selector.
func selectTaskForMove(project *models.Project, args []string, action string) (*models.Task, error) {
	var filter TaskFilter
	switch action {
	case "up":
		filter = CanMoveUpFilter(project)
	case "down":
		filter = CanMoveDownFilter(project)
	case "indent":
		filter = CanIndentFilter(project)
	case "outdent":
		filter = CanOutdentFilter()
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}

	prompt := fmt.Sprintf("Select task to move %s:", action)
	return SelectTask(project, args, filter, prompt, "no tasks available for this action")
}
