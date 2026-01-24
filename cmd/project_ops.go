package cmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
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
				evalCtx := BuildEvaluationContext(project.Tasks, opts.PrevStates)
				result = workflow.EvaluateRulesWithContext(evalCtx, rules)
			} else {
				// Use standard evaluation (no event triggers)
				result = workflow.EvaluateRules(project.Tasks, rules)
			}

			if len(result.Conflicts) > 0 {
				fmt.Fprintln(os.Stderr, "Warning: workflow rule conflicts detected:")
				fmt.Fprintln(os.Stderr, workflow.FormatConflicts(result.Conflicts))
			}
			// Persist trigger counts if any rules fired
			if len(result.RuleTriggerCounts) > 0 {
				Debugf("workflow rules triggered: %v", result.RuleTriggerCounts)
				opts.Config.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
				if err := opts.ConfigManager.Save(opts.Config); err != nil {
					fmt.Fprintln(os.Stderr, "Warning: failed to save rule trigger counts:", err)
				}
			}
		}
	}

	project.ReorderRootTasksByCompletion()

	Debugf("saving project: %s", project.Name)
	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return storage.SaveProject(project)
}

// SaveProjectWithBackup saves the project after creating a backup.
// It also runs auto-unblock checks and reorders root tasks by completion status.
// Deprecated: Use SaveProject with nil opts instead.
func SaveProjectWithBackup(project *models.Project) error {
	return SaveProject(project, nil)
}

// SaveProjectWithRules saves the project after evaluating workflow rules.
// It runs auto-unblock checks, evaluates rules, creates a backup, and saves.
// Prints warnings to stderr if there are rule conflicts.
// If configManager is provided, trigger counts are persisted.
// This version does not support event-based triggers (use SaveProjectWithRulesAndChanges).
// Deprecated: Use SaveProject with SaveOptions instead.
func SaveProjectWithRules(project *models.Project, cfg *config.Config, configManager *config.Manager) error {
	return SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
	})
}

// SaveProjectWithRulesAndChanges saves the project after evaluating workflow rules.
// If prevStates is provided, it enables event-based triggers by comparing current
// state against previous state. Pass nil for prevStates to disable event triggers.
// Deprecated: Use SaveProject with SaveOptions instead.
func SaveProjectWithRulesAndChanges(project *models.Project, cfg *config.Config, configManager *config.Manager, prevStates map[string]*models.Task) error {
	return SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
		PrevStates:    prevStates,
	})
}

// GetProjectFromSelector safely extracts a Project from a SelectorItem.
// Returns an error if the data is not a *models.Project.
func GetProjectFromSelector(item *tui.SelectorItem) (*models.Project, error) {
	project, ok := item.Data.(*models.Project)
	if !ok {
		return nil, fmt.Errorf("internal error: expected Project, got %T", item.Data)
	}
	return project, nil
}

// ProjectsToSelectorItems converts projects to selector items.
// If includeActive is true, includes the "Active" column.
func ProjectsToSelectorItems(projects []*models.Project, includeActive bool) []tui.SelectorItem {
	var headers []string
	if includeActive {
		headers = []string{"Project", "Context", "Tasks", "Active"}
	} else {
		headers = []string{"Project", "Context", "Tasks"}
	}

	items := make([]tui.SelectorItem, len(projects))
	for i, p := range projects {
		columns := []string{
			p.Name,
			p.Context,
			fmt.Sprintf("%d tasks", p.CountTasks()),
		}
		if includeActive {
			columns = append(columns, fmt.Sprintf("%d active", p.CountActiveTasks()))
		}
		items[i] = tui.SelectorItem{
			ID:      p.ID,
			Data:    p,
			Headers: headers,
			Columns: columns,
		}
	}
	return items
}
