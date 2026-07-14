package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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

	taskMoveUpCmd.ValidArgsFunction = completeCurrentProjectTaskIDs()
	taskMoveDownCmd.ValidArgsFunction = completeCurrentProjectTaskIDs()
	taskIndentCmd.ValidArgsFunction = completeCurrentProjectTaskIDs()
	taskOutdentCmd.ValidArgsFunction = completeCurrentProjectTaskIDs()
}

// runTaskHierarchyMove is the shared skeleton for the sibling-ordering and
// indent/outdent subcommands. Interactive selection stays at the CLI edge (it needs
// the loaded project and the action-specific eligibility filter), but the mutation
// itself routes through the application layer so the reorder commits atomically,
// revision-checked, and undoably — with stale-revision retry — through one
// ChangeSet. The domain guards (already-at-top/bottom, illegal completion crossing,
// nesting-depth, type compatibility) live in the app use case, so an explicit ID
// that is ineligible still produces the user-facing error.
func runTaskHierarchyMove(cmd *cobra.Command, args []string, action, verb string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	project, workspace, err := LoadRuntimeCurrentProject()
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

	svc := newWorkspaceAppService(workspace)
	res, err := withTaskRetry(func() (app.TaskResult, error) {
		switch action {
		case "up":
			return svc.MoveTaskUp(task.ID)
		case "down":
			return svc.MoveTaskDown(task.ID)
		case "indent":
			return svc.IndentTask(task.ID)
		case "outdent":
			return svc.OutdentTask(task.ID)
		default:
			return app.TaskResult{}, fmt.Errorf("unknown action: %s", action)
		}
	})
	if err != nil {
		return translateAppError(err)
	}
	return emitMutation(out, verb, res)
}

func runTaskMoveUp(cmd *cobra.Command, args []string) error {
	return runTaskHierarchyMove(cmd, args, "up", "Moved up")
}

func runTaskMoveDown(cmd *cobra.Command, args []string) error {
	return runTaskHierarchyMove(cmd, args, "down", "Moved down")
}

func runTaskIndent(cmd *cobra.Command, args []string) error {
	return runTaskHierarchyMove(cmd, args, "indent", "Indented")
}

func runTaskOutdent(cmd *cobra.Command, args []string) error {
	return runTaskHierarchyMove(cmd, args, "outdent", "Outdented")
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
