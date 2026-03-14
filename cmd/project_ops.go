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
	// Config is required when EvaluateRules is true.
	Config *config.Config
	// ConfigManager is required when EvaluateRules is true.
	ConfigManager *config.Manager
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
		}
	}

	project.ReorderRootTasksByCompletion()

	Debugf("saving project: %s", project.Name)
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		return fmt.Errorf("failed to ensure project directory: %w", err)
	}
	lockPath := filepath.Join(project.Path, "tasks.md.lock")
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

		// Find and remove the task from source
		var movedTask *models.Task
		var remaining []*models.Task
		for _, t := range srcProject.Tasks {
			if t.ID == move.TaskID {
				movedTask = t
			} else {
				remaining = append(remaining, t)
			}
		}
		if movedTask == nil {
			continue // task already removed (e.g., by a prior move)
		}

		srcProject.Tasks = remaining
		srcProject.BuildTaskTree()

		// Clear cross-project references on the moved task
		movedTask.DependsOn = nil
		movedTask.RelatedTo = nil

		// Clear references in source project that point to the moved task
		for _, t := range srcProject.Tasks {
			t.RemoveDependency(movedTask.ID)
			t.RemoveRelatedTo(movedTask.ID)
		}

		// Add to destination with a new short ID
		movedTask.ShortID = destProject.GetNextShortID()
		movedTask.ParentID = nil // clear parent since it's in a new project
		movedTask.Parent = nil
		movedTask.Children = nil
		destProject.Tasks = append(destProject.Tasks, movedTask)
		destProject.BuildTaskTree()

		// Save destination project
		if err := storage.BackupProject(destProject); err != nil {
			return fmt.Errorf("failed to backup destination project %q: %w", move.Destination, err)
		}
		if err := storage.SaveProject(destProject); err != nil {
			return fmt.Errorf("failed to save destination project %q: %w", move.Destination, err)
		}

		Debugf("moved task %s to project %s", move.TaskID, move.Destination)
	}
	return nil
}
