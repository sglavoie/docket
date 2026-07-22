package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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

	addOutputFlags(taskPromoteCmd)
	addOutputFlags(taskDemoteCmd)
	addOutputFlags(taskCycleCmd)
}

func runTaskPromote(cmd *cobra.Command, args []string) error {
	return changeTaskType(cmd, args, "promote", app.PromoteType)
}

func runTaskDemote(cmd *cobra.Command, args []string) error {
	return changeTaskType(cmd, args, "demote", app.DemoteType)
}

func runTaskCycle(cmd *cobra.Command, args []string) error {
	change := app.CycleTypeRight
	if cycleLeft {
		change = app.CycleTypeLeft
	}
	return changeTaskType(cmd, args, "cycle", change)
}

// changeTaskType resolves the target task (by explicit ID or interactive selection)
// at the CLI edge, then routes the type-hierarchy mutation through the application
// layer so it commits atomically, revision-checked, and undoably — with
// stale-revision retry — as one ChangeSet. The type-compatibility guards live in
// the app use case, so an ineligible explicit ID still surfaces the model's reason.
func changeTaskType(cmd *cobra.Command, args []string, action string, change app.TaskTypeChange) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task
	if len(args) > 0 {
		var findErr error
		task, findErr = FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
	} else {
		selected, selErr := selectTaskForTypeChange(project, action)
		if selErr != nil {
			return selErr
		}
		task = selected
	}

	oldType := task.Type

	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
		return workflows.service.ChangeTaskTypeWithWorkflowByLocator(locator, change, workflows.options(true, typeChangeUndoAction(change), task))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	res := workflowResultTask(cascade)

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return emitMutation(out, "", res)
	}
	// The application layer mutates a freshly-snapshotted task, not the CLI's loaded
	// copy, so derive the new type from the (now-validated) change to render the
	// old→new transition. The change succeeded, so the preview matches the commit.
	newType := previewTypeChange(oldType, change)
	fmt.Printf("%s %s: %s (%s -> %s)\n",
		newType.HeadingPrefix(), res.QualifiedRef, res.Title, oldType, newType)
	return nil
}

func typeChangeUndoAction(change app.TaskTypeChange) string {
	switch change {
	case app.PromoteType:
		return "Promote Task"
	case app.DemoteType:
		return "Demote Task"
	case app.CycleTypeRight, app.CycleTypeLeft:
		return "Cycle Task Type"
	default:
		return "Change Task Type"
	}
}

// previewTypeChange returns the type a successful type change produces from the
// given starting type. It mirrors the pure model transitions the use case applied,
// so the CLI can render the old→new transition without re-reading the mutated task.
func previewTypeChange(from models.TaskType, change app.TaskTypeChange) models.TaskType {
	switch change {
	case app.PromoteType:
		return from.Promote()
	case app.DemoteType:
		return from.Demote()
	case app.CycleTypeRight:
		return from.CycleLeafRight()
	case app.CycleTypeLeft:
		return from.CycleLeafLeft()
	default:
		return from
	}
}

// selectTaskForTypeChange presents an interactive selector over the tasks eligible
// for the given type-change action.
func selectTaskForTypeChange(project *models.Project, action string) (*models.Task, error) {
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
		return nil, fmt.Errorf("no tasks available for this action")
	}

	items := TasksToSelectorItems(candidates)
	prompt := fmt.Sprintf("Select task to %s:", action)
	selected, err := tui.RunSelector(prompt, items)
	if err != nil {
		return nil, err
	}
	return GetTaskFromSelector(selected)
}
