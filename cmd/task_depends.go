package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	dependsOnFlag     string
	dependsRemoveFlag string
	dependsClearFlag  bool
)

var taskDependsCmd = &cobra.Command{
	Use:   "depends <id>",
	Short: "Manage task dependencies",
	Long: `View or manage dependencies for a task.

Without flags, shows the current dependencies.

Examples:
  tt task depends 5                    # Show dependencies for task 5
  tt task depends 5 --on 3             # Task 5 now depends on task 3
  tt task depends 5 --remove 3         # Remove dependency on task 3
  tt task depends 5 --clear            # Remove all dependencies`,
	RunE: runTaskDepends,
}

func init() {
	taskDependsCmd.Flags().StringVar(&dependsOnFlag, "on", "", "Add dependency on this task ID")
	taskDependsCmd.Flags().StringVar(&dependsRemoveFlag, "remove", "", "Remove dependency on this task ID")
	taskDependsCmd.Flags().BoolVar(&dependsClearFlag, "clear", false, "Clear all dependencies")

	taskCmd.AddCommand(taskDependsCmd)
	addOutputFlags(taskDependsCmd)
	_ = taskDependsCmd.RegisterFlagCompletionFunc("on", completeCurrentProjectTaskIDs())
	_ = taskDependsCmd.RegisterFlagCompletionFunc("remove", completeCurrentProjectTaskIDs())
}

func runTaskDepends(cmd *cobra.Command, args []string) error { //nolint:gocyclo
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("task ID")
	}

	taskID := args[0]

	// Mutual exclusion: only one of --clear, --remove, --on may be set
	flagCount := 0
	if dependsClearFlag {
		flagCount++
	}
	if dependsRemoveFlag != "" {
		flagCount++
	}
	if dependsOnFlag != "" {
		flagCount++
	}
	if flagCount > 1 {
		return fmt.Errorf("only one of --clear, --remove, --on can be used at a time")
	}

	// If no flags provided, just show dependencies (read-only)
	if !dependsClearFlag && dependsRemoveFlag == "" && dependsOnFlag == "" {
		project, _, err := LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		showTaskDependencies(project, task)
		return nil
	}

	// The dependency mutations resolve the acting task and blocker within the
	// current project (matching the legacy single-project reference rules) and
	// then persist through the application layer (app.Service) so the change
	// commits atomically, revision-checked, and undoably with stale-revision
	// retry. Dependency blocking is a DERIVED predicate, not a workflow state, so
	// removing/clearing the last dependency simply makes the task no longer
	// blocked (HasIncompleteDependencies becomes false) — the CLI no longer forces
	// a reopen/transition; the app layer preserves the task's workflow state.

	// Handle --clear
	if dependsClearFlag {
		project, workspace, err := LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		// Pre-check: skip the commit if there are no dependencies to clear.
		if len(task.DependsOn) == 0 {
			if !out.Quiet {
				fmt.Printf("Task #%d has no dependencies to clear.\n", task.ShortID)
			}
			return nil
		}

		workflows := newCLIWorkflowRunner(workspace)
		defer workflows.flush()
		cascade, err := withResultRetry(func() (app.BulkResult, error) {
			locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			return workflows.service.ClearDependenciesWithWorkflowByLocator(locator, workflows.options(true, "Clear Dependencies", task))
		})
		if err != nil {
			return translateAppError(err)
		}
		workflows.observe(cascade)
		if !workflowResultCommitted(cascade) {
			if !out.Quiet {
				fmt.Printf("Task #%d has no dependencies to clear.\n", task.ShortID)
			}
			return nil
		}
		res := workflowResultTask(cascade)
		if out.JSON {
			return emitMutation(out, "Cleared dependencies", res)
		}
		if !out.Quiet {
			fmt.Printf("Cleared all dependencies for #%d: %s\n", res.ShortID, res.Title)
		}
		return nil
	}

	// Handle --remove
	if dependsRemoveFlag != "" {
		project, workspace, err := LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		blocker, blockerErr := FindTaskByIdentifier(project, dependsRemoveFlag)
		if blockerErr != nil {
			return blockerErr
		}
		if !task.HasDependency(blocker.ID) {
			return fmt.Errorf("task #%d does not depend on #%d", task.ShortID, blocker.ShortID)
		}

		workflows := newCLIWorkflowRunner(workspace)
		defer workflows.flush()
		cascade, err := withResultRetry(func() (app.BulkResult, error) {
			locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			// A bare dependency reference is intentionally relative to the
			// locator-owned project, preserving the canonical same-project form.
			return workflows.service.RemoveDependencyWithWorkflowByLocator(locator, blocker.ID, workflows.options(true, "Remove Dependency", task))
		})
		if err != nil {
			return translateAppError(err)
		}
		workflows.observe(cascade)
		res := workflowResultTask(cascade)
		if out.JSON {
			return emitMutation(out, "Removed dependency", res)
		}
		if !out.Quiet {
			fmt.Printf("Removed dependency: #%d no longer depends on #%d (%s)\n",
				res.ShortID, blocker.ShortID, blocker.Title)
		}
		return nil
	}

	// Handle --on
	if dependsOnFlag != "" {
		project, workspace, err := LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		blocker, blockerErr := FindTaskByIdentifier(project, dependsOnFlag)
		if blockerErr != nil {
			return blockerErr
		}
		if blocker.ID == task.ID {
			return tterrors.ErrSelfDependency
		}
		if task.HasDependency(blocker.ID) {
			return fmt.Errorf("%w: #%d already depends on #%d", tterrors.ErrDuplicateDep, task.ShortID, blocker.ShortID)
		}
		if project.HasCircularDependency(task.ID, blocker.ID) {
			return fmt.Errorf("cannot add dependency: %w", tterrors.ErrCircularDep)
		}

		workflows := newCLIWorkflowRunner(workspace)
		defer workflows.flush()
		cascade, err := withResultRetry(func() (app.BulkResult, error) {
			locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			// A bare dependency reference is intentionally relative to the
			// locator-owned project, preserving the canonical same-project form.
			return workflows.service.AddDependencyWithWorkflowByLocator(locator, blocker.ID, workflows.options(true, "Add Dependency", task))
		})
		if err != nil {
			return translateAppError(err)
		}
		workflows.observe(cascade)
		res := workflowResultTask(cascade)
		if out.JSON {
			return emitMutation(out, "Added dependency", res)
		}
		if !out.Quiet {
			fmt.Printf("Added dependency: #%d now depends on #%d (%s)\n",
				res.ShortID, blocker.ShortID, blocker.Title)
		}
		return nil
	}

	return nil
}

func showTaskDependencies(project *models.Project, task *models.Task) {
	fmt.Printf("#%d: %s\n", task.ShortID, task.Title)
	fmt.Println(strings.Repeat("-", 40))

	// Show what this task depends on (local + cross-project)
	blockers := project.GetBlockingTasks(task)
	var crossProjectDeps []models.TaskRef
	for _, depID := range task.DependsOn {
		ref := models.ParseTaskRef(depID)
		if ref.IsCrossProject() {
			crossProjectDeps = append(crossProjectDeps, ref)
		}
	}
	if len(blockers) == 0 && len(crossProjectDeps) == 0 {
		fmt.Println("Depends on: (none)")
	} else {
		fmt.Println("Depends on:")
		for _, b := range blockers {
			status := b.State().Icon()
			fmt.Printf("  %s #%d: %s\n", status, b.ShortID, b.Title)
		}
		for _, ref := range crossProjectDeps {
			fmt.Printf("  ⧉ %s (cross-project)\n", ref.String())
		}
	}

	// Show what depends on this task
	blocked := project.GetBlockedByTasks(task)
	if len(blocked) == 0 {
		fmt.Println("Blocks: (none)")
	} else {
		fmt.Println("Blocks:")
		for _, t := range blocked {
			status := t.State().Icon()
			fmt.Printf("  %s #%d: %s\n", status, t.ShortID, t.Title)
		}
	}
}
