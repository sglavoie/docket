package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/tui"
)

var taskMoveToForce bool

var taskMoveToCmd = &cobra.Command{
	Use:   "move-to [task-id] [dest-project]",
	Short: "Move a task to another project",
	Long: `Move a task and all its children to a different project.

The task becomes a root task in the destination project. All descendants
move together, preserving their hierarchy.

Dependencies pointing to tasks that remain in the source project are cleared.
Use --force to skip the confirmation when dependencies will be cleared.

Short IDs are regenerated in the destination project.
Canonical workflow state is preserved.

Examples:
  tt task move-to 5 work        # Move task #5 to the "work" project
  tt task move-to 5 work -f     # Move without confirmation even if deps cleared`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runTaskMoveTo,
}

func init() {
	taskCmd.AddCommand(taskMoveToCmd)
	taskMoveToCmd.Flags().BoolVarP(&taskMoveToForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(taskMoveToCmd)
	taskMoveToCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return completeProjectSlugs()(nil, nil, "")
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTaskMoveTo(cmd *cobra.Command, args []string) error { //nolint:gocyclo
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}

	sourceProject := workspace.Runtime.CurrentProject()
	if sourceProject == nil {
		return NoCurrentProjectError()
	}

	if loadErr := storage.LoadProject(sourceProject); loadErr != nil {
		return fmt.Errorf("failed to load source project: %w", loadErr)
	}

	// Select task if not provided
	var taskID string
	if len(args) >= 1 {
		taskID = args[0]
	} else {
		// Interactive selection
		if len(sourceProject.Tasks) == 0 {
			return fmt.Errorf("%s", NoItemsMsg("tasks", "tt task add <title>"))
		}
		items := TasksToSelectorItems(sourceProject.Tasks)
		selected, selErr := tui.RunSelector("Select task to move:", items)
		if selErr != nil {
			return selErr
		}
		task, getErr := GetTaskFromSelector(selected)
		if getErr != nil {
			return getErr
		}
		taskID = fmt.Sprintf("%d", task.ShortID)
	}

	task, findErr := FindTaskByIdentifier(sourceProject, taskID)
	if findErr != nil {
		return findErr
	}

	// Select destination project
	var destProjectID string
	if len(args) >= 2 {
		destProjectID = args[1]
	} else {
		// Interactive selection - exclude current project
		var candidates []*struct {
			Project any
			ID      string
			Name    string
			Context string
			Tasks   int
		}
		active := workspace.Runtime.ActiveProjects()
		for _, p := range active {
			if p.ID != sourceProject.ID {
				taskCount := 0
				if loadErr := storage.LoadProject(p); loadErr == nil {
					taskCount = p.CountTasks()
				}
				candidates = append(candidates, &struct {
					Project any
					ID      string
					Name    string
					Context string
					Tasks   int
				}{
					Project: p,
					ID:      p.ID,
					Name:    p.Name,
					Context: p.Context,
					Tasks:   taskCount,
				})
			}
		}
		if len(candidates) == 0 {
			return fmt.Errorf("%s", NoItemsMsg("other projects", "tt project new <name>"))
		}

		// Build selector items
		items := make([]selector.Item, len(candidates))
		for i, c := range candidates {
			items[i] = selector.Item{
				ID:      c.ID,
				Data:    c.Project,
				Headers: []string{"Project", "Context", "Tasks"},
				Columns: []string{c.Name, c.Context, fmt.Sprintf("%d", c.Tasks)},
			}
		}

		selected, selErr := tui.RunSelector("Select destination project:", items)
		if selErr != nil {
			return selErr
		}
		destProjectID = selected.ID
	}

	destProject := workspace.Runtime.GetProject(destProjectID)
	if destProject == nil {
		return fmt.Errorf("destination %w", tterrors.NewProjectNotFoundError(destProjectID))
	}
	if destProject.ID == sourceProject.ID {
		return fmt.Errorf("cannot move task to the same project")
	}

	if loadErr := storage.LoadProject(destProject); loadErr != nil {
		return fmt.Errorf("failed to load destination project: %w", loadErr)
	}

	// Build the moved set (root + descendants) from the loaded source project so the
	// dep-clear confirmation and durable planning reassignment can be computed at the
	// CLI edge before the application layer performs the move. Task UUIDs are stable
	// across the move, so this set keys the config reassignment correctly regardless
	// of the fresh snapshot the use case loads.
	sourceProject.BuildTaskTree()
	movedSet := make(map[string]bool)
	var collectMovedIDs func(taskID string)
	collectMovedIDs = func(taskID string) {
		t := sourceProject.GetTaskByID(taskID)
		if t == nil {
			return
		}
		movedSet[t.ID] = true
		for _, child := range t.Children {
			collectMovedIDs(child.ID)
		}
	}
	collectMovedIDs(task.ID)

	// Check for dependencies that will be cleared. Cross-project refs
	// ("projectID:taskID") are preserved during the move, so only count same-project
	// refs that point outside the moved set.
	var depsToBeCleared int
	for id := range movedSet {
		t := sourceProject.GetTaskByID(id)
		if t == nil {
			continue
		}
		for _, depID := range t.DependsOn {
			if models.ParseTaskRef(depID).IsCrossProject() {
				continue // cross-project refs are preserved, not cleared
			}
			if !movedSet[depID] {
				depsToBeCleared++
			}
		}
	}

	// Confirm if dependencies will be cleared
	if depsToBeCleared > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d dependencies will be cleared (they point to tasks not being moved)\n", depsToBeCleared)
		if !ConfirmYN("Continue? [y/N]: ", taskMoveToForce) {
			if !out.Quiet {
				fmt.Fprintf(os.Stderr, "Canceled\n")
			}
			return nil
		}
	}

	// Reassign the DURABLE planning refs (Focus Queue, Daily Plan) to the destination
	// project and fold them into the move transaction as a config artifact, so a
	// partial move can never leave planning refs pointing at the old project. The
	// machine-local recent visits are intentionally excluded here and reassigned
	// separately after a successful commit.
	var artifacts []storage.ArtifactChange
	if domains.Planning.ReassignDurablePlanningProjectFrom(movedSet, sourceProject.ID, destProject.ID) > 0 {
		arts, artErr := encodeSyncedDomainArtifacts(workspace.Runtime, domains)
		if artErr != nil {
			return artErr
		}
		artifacts = arts
	}

	// Route the move through the application layer: source removal, destination
	// insertion, each moved task's attachment directory, and the durable planning
	// artifact all commit as ONE transaction, revision-checked and undoable.
	svc := newWorkspaceAppService(workspace)
	res, err := svc.MoveToProjectByLocator(app.TaskLocator{ProjectID: sourceProject.ID, TaskID: task.ID}, destProject.ID, artifacts...)
	if err != nil {
		return translateAppError(err)
	}

	// Repair the machine-local recent visits separately — non-authoritative UI state
	// updated only after the durable commit succeeded. A failure here is a warning,
	// not an error: the move is already committed and recent visits are best-effort.
	if domains.Machine.ReassignRecentVisitsProjectFrom(movedSet, sourceProject.ID, destProject.ID) > 0 {
		if saveErr := workspace.Runtime.CommitMachine(domains.Machine); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update recent visits: %v\n", saveErr)
		}
	}

	if err := emitMutation(out, "Moved", res.Task); err != nil {
		return err
	}
	if !out.Quiet && !out.JSON {
		if descendants := res.MovedCount - 1; descendants > 0 {
			fmt.Printf("  Moved %d descendant(s) with it.\n", descendants)
		}
		if len(res.ClearedDeps) > 0 {
			fmt.Printf("  Cleared %d dependencies that pointed to tasks in the source project.\n", len(res.ClearedDeps))
		}
	}
	return nil
}
