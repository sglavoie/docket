package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workflow"
)

// SaveOptions configures the behavior of SaveProject.
type SaveOptions struct {
	// EvaluateRules enables workflow rule evaluation before saving.
	EvaluateRules bool
	// PrevStates provides previous task states for event-based triggers.
	// If nil, event triggers are disabled.
	PrevStates map[string]*models.Task
	// Config is required when EvaluateRules is true (and for undo depth).
	Config *config.Config
	// ConfigManager is required when EvaluateRules is true.
	ConfigManager *config.Manager
	// UndoAction describes the action for undo history (e.g., "Add Task").
	// If empty, no undo snapshot is recorded.
	UndoAction string
	// UndoContext provides additional context for the undo entry (e.g., task title).
	UndoContext string
}

// SaveProject saves the project with optional workflow rule evaluation.
// If opts is nil, performs basic backup and save with auto-unblock and reordering.
// If opts.EvaluateRules is true, evaluates workflow rules before saving.
// Prints warnings to stderr if there are rule conflicts.
func SaveProject(project *models.Project, opts *SaveOptions) error {
	project.CheckAndAutoUnblock()

	// Evaluate workflow rules if requested
	if opts != nil && opts.EvaluateRules {
		if opts.Config == nil || opts.ConfigManager == nil {
			return fmt.Errorf("config and ConfigManager required when EvaluateRules is true")
		}

		rules := opts.Config.GetEnabledWorkflowRules()
		if len(rules) > 0 {
			Debugf("evaluating %d workflow rules", len(rules))
			var result *workflow.EvaluationResult

			if opts.PrevStates != nil {
				// Use context-aware evaluation with event triggers
				evalCtx := BuildEvaluationContext(project.Tasks, opts.PrevStates, project.Name, project.ID)
				result = workflow.EvaluateRulesWithContext(evalCtx, rules)
			} else {
				// Use standard evaluation (no event triggers)
				result = workflow.EvaluateRules(project.Tasks, rules)
			}

			if len(result.Conflicts) > 0 {
				logging.Warn("workflow rule conflicts detected:\n%s", workflow.FormatConflicts(result.Conflicts))
			}

			// Process follow-up tasks created by create_followup actions
			for _, followup := range result.NewFollowupTasks {
				followup.ShortID = project.GetNextShortID()
				project.Tasks = append(project.Tasks, followup)
			}
			if len(result.NewFollowupTasks) > 0 {
				project.BuildTaskTree()
				Debugf("created %d follow-up tasks", len(result.NewFollowupTasks))
			}

			// Process deferred moves (move tasks to other projects)
			if len(result.DeferredMoves) > 0 {
				if err := processDeferredMoves(project, result.DeferredMoves, opts.Config, opts.ConfigManager); err != nil {
					return fmt.Errorf("workflow move_to_project failed: %w", err)
				}
			}

			// Persist trigger counts if any rules fired
			if len(result.RuleTriggerCounts) > 0 {
				Debugf("workflow rules triggered: %v", result.RuleTriggerCounts)
				opts.Config.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
				if err := opts.ConfigManager.Save(opts.Config); err != nil {
					logging.Warn("failed to save rule trigger counts: %v", err)
				}
			}

			// Evaluate project-level rules against aggregate task data
			projResult := evaluateAndApplyProjectRules(project, opts.Config, rules)
			if projResult.ProjectStatusSet {
				Debugf("project status set to %q by rule %q", projResult.ProjectStatusIcon, projResult.ProjectRuleName)
				if len(projResult.TriggeredRuleIDs) > 0 {
					counts := make(map[string]int, len(projResult.TriggeredRuleIDs))
					for _, id := range projResult.TriggeredRuleIDs {
						counts[id]++
					}
					opts.Config.IncrementRuleTriggerCounts(counts)
					if err := opts.ConfigManager.Save(opts.Config); err != nil {
						logging.Warn("failed to save project rule trigger counts: %v", err)
					}
				}
			}
		}
	}

	// Re-run post-mutation hooks after workflow rules: rules may have set
	// tasks to "done" (e.g., auto-complete parent), which requires generating
	// recurring instances and unblocking dependents.
	project.CheckAndAutoUnblock()
	project.GenerateRecurringInstances()

	project.ReorderRootTasksByCompletion()

	// Record undo snapshot before backup+save so it captures the pre-mutation state
	if opts != nil && opts.UndoAction != "" && opts.Config != nil && opts.Config.UndoHistoryDepth != nil && *opts.Config.UndoHistoryDepth > 0 {
		if err := storage.RecordUndoSnapshot(project, opts.UndoAction, opts.UndoContext, *opts.Config.UndoHistoryDepth); err != nil {
			logging.Warn("failed to record undo snapshot: %v", err)
		}
	}

	Debugf("saving project: %s", project.Name)
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		return fmt.Errorf("failed to ensure project directory: %w", err)
	}
	lockPath := filepath.Join(project.Path, storage.LockFileName)
	return util.WithFileLock(lockPath, true, func() error {
		if err := storage.BackupProject(project); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		return storage.SaveProject(project)
	})
}

// GetProjectFromSelector safely extracts a Project from a selector.Item.
// Returns an error if the data is not a *models.Project.
func GetProjectFromSelector(item *selector.Item) (*models.Project, error) {
	project, ok := item.Data.(*models.Project)
	if !ok {
		return nil, fmt.Errorf("internal error: expected Project, got %T", item.Data)
	}
	return project, nil
}

// ProjectsToSelectorItems converts projects to selector items.
// If includeActive is true, includes the "Active" column.
func ProjectsToSelectorItems(projects []*models.Project, includeActive bool) []selector.Item {
	var headers []string
	if includeActive {
		headers = []string{"Project", "Context", "Tasks", "Active"}
	} else {
		headers = []string{"Project", "Context", "Tasks"}
	}

	items := make([]selector.Item, len(projects))
	for i, p := range projects {
		columns := []string{
			p.Name,
			p.Context,
			fmt.Sprintf("%d tasks", p.CountTasks()),
		}
		if includeActive {
			columns = append(columns, fmt.Sprintf("%d active", p.CountActiveTasks()))
		}
		items[i] = selector.Item{
			ID:      p.ID,
			Data:    p,
			Headers: headers,
			Columns: columns,
		}
	}
	return items
}

// processDeferredMoves handles move_to_project actions by transferring tasks
// from the source project to destination projects.
// Validates all destinations exist and are loadable before making any changes,
// so that a missing destination doesn't leave the source project partially modified.
func processDeferredMoves(srcProject *models.Project, moves []workflow.DeferredMove, cfg *config.Config, _ *config.Manager) error {
	// Phase 1: Validate and load all destination projects upfront.
	destProjects := make(map[string]*models.Project, len(moves))
	for _, move := range moves {
		if _, ok := destProjects[move.Destination]; ok {
			continue // already validated
		}
		destProject := cfg.GetProject(move.Destination)
		if destProject == nil {
			return fmt.Errorf("destination project %q not found", move.Destination)
		}
		if err := storage.LoadProject(destProject); err != nil {
			return fmt.Errorf("failed to load destination project %q: %w", move.Destination, err)
		}
		destProjects[move.Destination] = destProject
	}

	// Phase 2: All destinations valid — perform the moves.
	for _, move := range moves {
		destProject := destProjects[move.Destination]

		// Find the root task in source
		var movedTask *models.Task
		for _, t := range srcProject.Tasks {
			if t.ID == move.TaskID {
				movedTask = t
				break
			}
		}
		if movedTask == nil {
			continue // task already removed (e.g., by a prior move)
		}

		// Collect the full subtree (task + all descendants) so children move
		// with their parent, matching the behavior of MoveTaskToProject.
		// Walk by ParentID rather than Children to avoid requiring BuildTaskTree
		// (which would also clean orphaned deps before we can warn about them).
		movedIDs := make(map[string]bool)
		movedIDs[movedTask.ID] = true
		changed := true
		for changed {
			changed = false
			for _, t := range srcProject.Tasks {
				if movedIDs[t.ID] {
					continue
				}
				if t.ParentID != nil && movedIDs[*t.ParentID] {
					movedIDs[t.ID] = true
					changed = true
				}
			}
		}

		// Partition source tasks into moved and remaining
		var movedTasks []*models.Task
		var remaining []*models.Task
		for _, t := range srcProject.Tasks {
			if movedIDs[t.ID] {
				movedTasks = append(movedTasks, t)
			} else {
				remaining = append(remaining, t)
			}
		}

		srcProject.Tasks = remaining
		srcProject.BuildTaskTree()

		// Warn about cross-project references being dropped
		for _, mt := range movedTasks {
			if len(mt.DependsOn) > 0 {
				fmt.Printf("warning: dropping %d dependency(s) for task #%d: %v\n", len(mt.DependsOn), mt.ShortID, mt.DependsOn)
			}
			if len(mt.RelatedTo) > 0 {
				fmt.Printf("warning: dropping %d relation(s) for task #%d: %v\n", len(mt.RelatedTo), mt.ShortID, mt.RelatedTo)
			}
		}

		// Clear cross-project references on all moved tasks
		for _, mt := range movedTasks {
			mt.DependsOn = []string{}
			mt.RelatedTo = []string{}
		}

		// Clear references in source project that point to any moved task
		for _, t := range srcProject.Tasks {
			for id := range movedIDs {
				t.RemoveDependency(id)
				t.RemoveRelatedTo(id)
			}
		}

		// Add tasks to destination with new short IDs
		for _, mt := range movedTasks {
			mt.ShortID = destProject.GetNextShortID()
		}
		movedTask.ParentID = nil // root task loses its parent in destination
		movedTask.Parent = nil
		destProject.Tasks = append(destProject.Tasks, movedTasks...)
		destProject.BuildTaskTree()

		// Save destination project with file locking to prevent corruption
		destLockPath := filepath.Join(destProject.Path, storage.LockFileName)
		if err := util.WithFileLock(destLockPath, true, func() error {
			if err := storage.BackupProject(destProject); err != nil {
				return fmt.Errorf("failed to backup destination project %q: %w", move.Destination, err)
			}
			if err := storage.SaveProject(destProject); err != nil {
				return fmt.Errorf("failed to save destination project %q: %w", move.Destination, err)
			}
			return nil
		}); err != nil {
			return err
		}

		Debugf("moved task %s to project %s", move.TaskID, move.Destination)
	}
	return nil
}
