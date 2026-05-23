package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var taskDeleteForce bool

var taskDeleteCmd = &cobra.Command{
	Use:   "delete [task-id]",
	Short: "Permanently delete a task",
	Long: `Permanently delete a task and all its descendants.

This removes the task from the project entirely. This cannot be undone
(unless undo history is enabled). Dangling references in DependsOn and
RelatedTo fields of other tasks are cleaned up automatically.

If no task ID is provided, an interactive selector is shown.

Examples:
  tt task delete 42
  tt task delete 42 --force
  tt t delete abc123`,
	RunE: runTaskDelete,
}

func init() {
	taskCmd.AddCommand(taskDeleteCmd)
	taskDeleteCmd.Flags().BoolVarP(&taskDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(taskDeleteCmd)
}

func runTaskDelete(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	// Select task: from args or interactive
	task, err := SelectTask(project, args, func(t *models.Task) bool {
		return true // all tasks are deletable
	}, "Select task to delete", "No tasks in this project.")
	if err != nil {
		return err
	}

	// Count descendants
	idsToDelete := collectTaskDescendantIDs(project.Tasks, map[string]bool{task.ID: true})
	count := len(idsToDelete)

	prompt := fmt.Sprintf("Permanently delete task #%d '%s'?", task.ShortID, task.Title)
	if count > 1 {
		prompt = fmt.Sprintf("Permanently delete task #%d '%s' and %s?",
			task.ShortID, task.Title, Pluralize(count-1, "descendant", "descendants"))
	}

	var confirmed bool
	if count > 1 {
		// ConfirmStrict prints its own "Type 'yes' to confirm:" prompt
		confirmed = ConfirmStrict(prompt, taskDeleteForce)
	} else {
		confirmed = ConfirmYN(prompt+" [y/N] ", taskDeleteForce)
	}
	if !confirmed {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Record undo snapshot
	if cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
		if snapshotErr := storage.RecordUndoSnapshot(project, "Delete Task",
			fmt.Sprintf("#%d %s", task.ShortID, task.Title), *cfg.UndoHistoryDepth, nil); snapshotErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to record undo snapshot: %v\n", snapshotErr)
		}
	}

	// Clean up attachments for all tasks being deleted
	for _, t := range project.Tasks {
		if idsToDelete[t.ID] {
			if cleanErr := storage.CleanupTaskAttachments(project, t); cleanErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to clean up attachments for task #%d: %v\n", t.ShortID, cleanErr)
			}
		}
	}

	// Filter out deleted tasks and clear dangling references
	project.Tasks = filterDeletedTasks(project.Tasks, idsToDelete)
	project.BuildTaskTree()

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	// for all deleted tasks including descendants.
	if cfg.RemoveTaskRefs(idsToDelete) > 0 {
		if err := configManager.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save config after cleaning task refs: %v\n", err)
		}
	}

	// Save (undo snapshot already recorded above, don't duplicate it via SaveOptions)
	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
	}); err != nil {
		return err
	}

	// Clean cross-project references pointing to the deleted tasks from other projects.
	if err := cleanDeletedTaskCrossProjectRefs(cfg.Projects, project.ID, project.Slug, idsToDelete); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to clean cross-project references: %v\n", err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	msg := TaskMsg(project.Name, "Deleted", task)
	if count > 1 {
		msg += fmt.Sprintf(" and %s", Pluralize(count-1, "descendant", "descendants"))
	}
	fmt.Println(msg)
	return nil
}

// cleanDeletedTaskCrossProjectRefs removes cross-project references to the deleted tasks
// from all other projects. deletedProjectID is the UUID of the source project (used to
// skip it), deletedProjectSlug is the slug used in cross-project ref format ("slug:taskID"),
// and deletedTaskIDs is the set of task IDs that were deleted.
func cleanDeletedTaskCrossProjectRefs(projects []*models.Project, deletedProjectID, deletedProjectSlug string, deletedTaskIDs map[string]bool) error {
	for _, p := range projects {
		if p.ID == deletedProjectID {
			continue
		}
		if err := storage.LoadProject(p); err != nil {
			return fmt.Errorf("loading project '%s': %w", p.Name, err)
		}
		cleaned := 0
		for _, t := range p.Tasks {
			orig := len(t.DependsOn)
			t.DependsOn = filterDeletedTaskRefs(t.DependsOn, deletedProjectSlug, deletedTaskIDs)
			depsRemoved := orig - len(t.DependsOn)
			cleaned += depsRemoved
			// If a Blocked task's entire DependsOn list was just removed
			// (all blockers were deleted), reopen it to prevent it being stuck.
			if t.Status == models.StatusBlocked && depsRemoved > 0 && len(t.DependsOn) == 0 {
				t.Reopen()
			}

			orig = len(t.RelatedTo)
			t.RelatedTo = filterDeletedTaskRefs(t.RelatedTo, deletedProjectSlug, deletedTaskIDs)
			cleaned += orig - len(t.RelatedTo)
		}
		if cleaned > 0 {
			if err := storage.SaveProjectLocked(p); err != nil {
				return fmt.Errorf("saving project '%s': %w", p.Name, err)
			}
		}
	}
	return nil
}

// filterDeletedTaskRefs removes cross-project refs to deleted tasks in the given project slug.
func filterDeletedTaskRefs(refs []string, projectSlug string, deletedTaskIDs map[string]bool) []string {
	if len(refs) == 0 {
		return refs
	}
	filtered := refs[:0]
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() && parsed.ProjectID == projectSlug && deletedTaskIDs[parsed.TaskID] {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
}

// filterDeletedTasks removes tasks in idsToDelete from the task list and cleans
// up dangling ParentID, DependsOn, and RelatedTo references in surviving tasks.
func filterDeletedTasks(tasks []*models.Task, idsToDelete map[string]bool) []*models.Task {
	var remaining []*models.Task
	for _, t := range tasks {
		if idsToDelete[t.ID] {
			continue
		}
		if t.ParentID != nil && idsToDelete[*t.ParentID] {
			t.ParentID = nil
		}
		if len(t.DependsOn) > 0 {
			cleaned := t.DependsOn[:0]
			for _, depID := range t.DependsOn {
				if !idsToDelete[depID] {
					cleaned = append(cleaned, depID)
				}
			}
			// A Blocked task whose entire DependsOn list was just deleted
			// has no remaining reason to stay blocked. Reopen it so the
			// user is not left with a task stuck in Blocked with no deps.
			if t.Status == models.StatusBlocked && len(cleaned) == 0 {
				t.Reopen()
			}
			t.DependsOn = cleaned
		}
		if len(t.RelatedTo) > 0 {
			cleaned := t.RelatedTo[:0]
			for _, relID := range t.RelatedTo {
				if !idsToDelete[relID] {
					cleaned = append(cleaned, relID)
				}
			}
			t.RelatedTo = cleaned
		}
		remaining = append(remaining, t)
	}
	return remaining
}

// collectTaskDescendantIDs collects IDs of root tasks and all their descendants.
// This is a cmd-level version of the TUI's collectDescendantIDs.
func collectTaskDescendantIDs(tasks []*models.Task, rootIDs map[string]bool) map[string]bool {
	idsToDelete := make(map[string]bool)
	var collectIDs func(t *models.Task)
	collectIDs = func(t *models.Task) {
		idsToDelete[t.ID] = true
		for _, child := range t.Children {
			collectIDs(child)
		}
	}
	for _, t := range tasks {
		if rootIDs[t.ID] {
			collectIDs(t)
		}
	}
	return idsToDelete
}
