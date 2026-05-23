package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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
Sections are preserved if they exist in the destination, otherwise reset to Backlog.

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

	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	sourceProject := cfg.GetCurrentProject()
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
		for _, p := range cfg.GetActiveProjects() {
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

	destProject := cfg.GetProject(destProjectID)
	if destProject == nil {
		return fmt.Errorf("destination %w", tterrors.NewProjectNotFoundError(destProjectID))
	}
	if destProject.ID == sourceProject.ID {
		return fmt.Errorf("cannot move task to the same project")
	}

	if loadErr := storage.LoadProject(destProject); loadErr != nil {
		return fmt.Errorf("failed to load destination project: %w", loadErr)
	}

	// Build tree to count descendants and check dependencies
	sourceProject.BuildTaskTree()

	// Build moved set to check for cross-project deps
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
	descendants := len(movedSet) - 1 // Exclude the root task itself

	// Check for dependencies that will be cleared
	// Cross-project refs (format "projectID:taskID") are preserved during the move,
	// so only count same-project refs that point outside the moved set.
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

	// Record undo snapshot for the source project before the move
	if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
		if snapshotErr := storage.RecordUndoSnapshot(sourceProject, "Move Task to Project",
			fmt.Sprintf("#%d %s → %s", task.ShortID, task.Title, destProject.Name), *cfg.UndoHistoryDepth, nil); snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot: %v\n", snapshotErr)
		}
	}

	// Perform the move
	result, err := sourceProject.MoveTaskToProject(task, destProject)
	if err != nil {
		return fmt.Errorf("failed to move task: %w", err)
	}
	if result == nil {
		return fmt.Errorf("move operation returned no result")
	}

	// Update FocusQueue, DailyPlan, and RecentTaskVisits to point to the new project
	movedTaskIDs := make(map[string]bool, len(result.MovedTasks))
	for _, t := range result.MovedTasks {
		movedTaskIDs[t.ID] = true
	}
	cfg.ReassignTaskProject(movedTaskIDs, destProject.ID)

	// Save both projects (undo snapshot already recorded above for source project)
	if err := SaveProject(sourceProject, &SaveOptions{Config: cfg, ConfigManager: configManager}); err != nil {
		return err
	}
	if err := SaveProject(destProject, &SaveOptions{Config: cfg, ConfigManager: configManager}); err != nil {
		return err
	}

	// Save config if needed (NextShortID is in tasks.md, not config)
	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		if descendants > 0 {
			fmt.Printf("Moved #%d: %s (and %d descendants) to %s\n",
				result.OldShortIDs[0], task.Title, descendants, destProject.Name)
		} else {
			fmt.Printf("Moved #%d: %s to %s\n",
				result.OldShortIDs[0], task.Title, destProject.Name)
		}
		if len(result.ClearedDeps) > 0 {
			fmt.Printf("Cleared %d dependencies that pointed to tasks in the source project.\n", len(result.ClearedDeps))
		}
		fmt.Printf("New task ID in %s: #%d\n", destProject.Name, task.ShortID)
	}
	return nil
}
