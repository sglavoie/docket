package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// TaskOpResult holds the outcome of a shared task operation.
type TaskOpResult struct {
	StatusMessage string
	Err           error
	Cmd           tea.Cmd
}

// persistFn is the callback that apply* functions use to look up a task,
// run the mutation, and persist the result. Different callers supply either
// a simple (backup→save→load) or a workflow-aware (auto-unblock, recurring,
// workflow rules, undo snapshots) implementation.
type persistFn func(project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult

// simplePersist returns a persistFn that wraps the existing withPersist helper.
// Used by secondary views that don't need the full workflow pipeline.
func simplePersist(p Persister) persistFn {
	return func(project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		return withPersist(p, project, taskID, mutate)
	}
}

// workflowPersist returns a persistFn that runs the full workflow pipeline
// (auto-unblock, recurring generation, workflow rules, root reorder, undo
// snapshots). The optsProvider is called at persist time so undo context
// is read fresh rather than captured at closure creation time.
func workflowPersist(optsProvider func() WorkflowSaveOptions) persistFn {
	return func(project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		if project.GetTaskByID(taskID) == nil {
			return TaskOpResult{StatusMessage: "Task no longer exists in project"}
		}
		prevStates := captureTaskStates(project.Tasks)
		project.MutateTask(taskID, mutate)
		opts := optsProvider()
		opts.PrevStates = prevStates
		if err := persistWithWorkflow(opts); err != nil {
			return TaskOpResult{Err: err}
		}
		return TaskOpResult{}
	}
}

// workflowOptsBuilder is implemented by views that provide workflow save options.
// ProjectTree and Outliner both satisfy this interface.
type workflowOptsBuilder interface {
	buildWorkflowOpts(prevStates map[string]*models.Task) WorkflowSaveOptions
}

// newWorkflowPersistFn returns a persistFn backed by the full workflow pipeline.
// It replaces the per-view workflowPersistFn() methods.
func newWorkflowPersistFn(v workflowOptsBuilder) persistFn {
	return workflowPersist(func() WorkflowSaveOptions {
		return v.buildWorkflowOpts(nil)
	})
}

// withPersist handles the common task operation pattern: look up task by ID,
// nil check, backup, apply mutation via MutateTask, save, and reload.
// On success it returns a zero TaskOpResult; callers set StatusMessage/Cmd.
func withPersist(p Persister, project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
	if project.GetTaskByID(taskID) == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if err := p.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}
	project.MutateTask(taskID, mutate)
	if err := p.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}
	_ = p.Load(project)
	return TaskOpResult{}
}

// WorkflowSaveOptions holds configuration for the save-with-workflow pipeline.
type WorkflowSaveOptions struct {
	Persister     Persister
	Project       *models.Project
	Config        *config.Config
	ConfigManager *config.Manager
	PrevStates    map[string]*models.Task
	UndoAction    string
	UndoContext   string
	UndoDepth     int
}

// BuildWorkflowOpts constructs a WorkflowSaveOptions from the given parameters.
// It resolves UndoDepth from the config when undo fields are provided.
func BuildWorkflowOpts(persister Persister, project *models.Project, cfg *config.Config, cfgManager *config.Manager, prevStates map[string]*models.Task, undoAction, undoContext string) WorkflowSaveOptions {
	opts := WorkflowSaveOptions{
		Persister:     persister,
		Project:       project,
		Config:        cfg,
		ConfigManager: cfgManager,
		PrevStates:    prevStates,
		UndoAction:    undoAction,
		UndoContext:   undoContext,
	}
	if cfg != nil && cfg.UndoHistoryDepth != nil {
		opts.UndoDepth = *cfg.UndoHistoryDepth
	}
	return opts
}

// persistWithWorkflow runs the full save pipeline used by both ProjectTree and Outliner:
// auto-unblock, recurring generation, workflow rules, root reorder, undo snapshot, backup, save, load.
func persistWithWorkflow(opts WorkflowSaveOptions) error {
	opts.Project.CheckAndAutoUnblock()
	opts.Project.GenerateRecurringInstances()

	if opts.Config != nil {
		rules := opts.Config.GetEnabledWorkflowRules()
		if len(rules) > 0 {
			var result *workflow.EvaluationResult
			if opts.PrevStates != nil {
				evalCtx := buildEvaluationContext(opts.Project.Tasks, opts.PrevStates, opts.Project.Name, opts.Project.ID)
				result = workflow.EvaluateRulesWithContext(evalCtx, rules)
			} else {
				result = workflow.EvaluateRules(opts.Project.Tasks, rules)
			}
			if len(result.Conflicts) > 0 {
				_ = result.Conflicts
			}

			// Process follow-up tasks created by create_followup actions
			for _, followup := range result.NewFollowupTasks {
				followup.ShortID = opts.Project.GetNextShortID()
				opts.Project.Tasks = append(opts.Project.Tasks, followup)
			}
			if len(result.NewFollowupTasks) > 0 {
				opts.Project.BuildTaskTree()
			}

			// Process deferred moves (move tasks to other projects)
			if len(result.DeferredMoves) > 0 {
				if err := processTUIDeferredMoves(opts.Persister, opts.Project, result.DeferredMoves, opts.Config); err != nil {
					return fmt.Errorf("workflow move_to_project failed: %w", err)
				}
			}

			if len(result.RuleTriggerCounts) > 0 && opts.ConfigManager != nil {
				opts.Config.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
				if err := opts.ConfigManager.Save(opts.Config); err != nil {
					logging.Warn("failed to save rule trigger counts: %v", err)
				}
			}

			// Evaluate project-level rules against aggregate task data
			if opts.Project.StatusIconSource != "user" {
				projCtx := &workflow.ProjectEvaluationContext{
					Project: opts.Project,
					Tasks:   opts.Project.Tasks,
				}
				projResult := workflow.EvaluateProjectRules(projCtx, rules)
				if projResult.NewStatusIcon != "" {
					opts.Project.StatusIcon = projResult.NewStatusIcon
					opts.Project.StatusIconSource = "auto"
					if len(projResult.TriggeredRuleIDs) > 0 && opts.ConfigManager != nil {
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
	}

	opts.Project.ReorderRootTasksByCompletion()

	if err := opts.Persister.Backup(opts.Project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	if err := opts.Persister.Save(opts.Project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	// Record undo snapshot only after successful save to avoid stale entries
	if opts.UndoDepth > 0 && opts.UndoAction != "" {
		_ = opts.Persister.RecordUndoSnapshot(opts.Project, opts.UndoAction, opts.UndoContext, opts.UndoDepth)
	}

	_ = opts.Persister.Load(opts.Project)
	return nil
}

// processTUIDeferredMoves handles move_to_project actions in the TUI context.
// It transfers tasks from the source project to destination projects.
func processTUIDeferredMoves(_ Persister, srcProject *models.Project, moves []workflow.DeferredMove, cfg *config.Config) error {
	for _, move := range moves {
		destProject := cfg.GetProject(move.Destination)
		if destProject == nil {
			return fmt.Errorf("destination project %q not found", move.Destination)
		}

		if err := storage.LoadProject(destProject); err != nil {
			return fmt.Errorf("failed to load destination project %q: %w", move.Destination, err)
		}

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
			continue
		}

		srcProject.Tasks = remaining
		srcProject.BuildTaskTree()

		movedTask.ShortID = destProject.GetNextShortID()
		movedTask.ParentID = nil
		movedTask.Parent = nil
		movedTask.Children = nil
		destProject.Tasks = append(destProject.Tasks, movedTask)
		destProject.BuildTaskTree()

		if err := storage.BackupProject(destProject); err != nil {
			return fmt.Errorf("failed to backup destination project %q: %w", move.Destination, err)
		}
		if err := storage.SaveProject(destProject); err != nil {
			return fmt.Errorf("failed to save destination project %q: %w", move.Destination, err)
		}
	}
	return nil
}

// performUndoFromBackup restores a project from backup using the given Persister.
// Returns an error message or nil on success.
func performUndoFromBackup(p Persister, project *models.Project) error {
	if !p.BackupExists(project) {
		return fmt.Errorf("no backup available to undo")
	}
	if err := p.RestoreFromBackup(project); err != nil {
		return fmt.Errorf("failed to undo: %w", err)
	}
	if err := p.Load(project); err != nil {
		return fmt.Errorf("failed to reload after undo: %w", err)
	}
	return nil
}

// performUndoCmd returns a tea.Cmd that restores the project from backup.
// The optional postUndo callback runs after a successful undo to let each view
// refresh its own data structures (e.g. rebuildRows, buildTree).
func performUndoCmd(persister Persister, project *models.Project, postUndo func()) tea.Cmd {
	return func() tea.Msg {
		if err := performUndoFromBackup(persister, project); err != nil {
			return ErrorMsg{Err: err}
		}
		if postUndo != nil {
			postUndo()
		}
		return UndoPerformedMsg{Action: "Undo"}
	}
}
