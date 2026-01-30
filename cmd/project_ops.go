package cmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/storage"
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
