package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

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

// simplePersistWithUndo returns a persistFn that records an undo snapshot
// before persisting. Used by secondary views that have config access but don't
// need the full workflow pipeline (e.g. pinned tasks popup, quick switch).
func simplePersistWithUndo(p Persister, cfg *config.Config, undoAction string) persistFn {
	return func(project *models.Project, taskID string, mutate func(*models.Task)) TaskOpResult {
		if cfg != nil && cfg.UndoHistoryDepth != nil && *cfg.UndoHistoryDepth > 0 {
			task := project.GetTaskByID(taskID)
			ctx := ""
			if task != nil {
				ctx = fmt.Sprintf("#%d %s", task.ShortID, task.Title)
			}
			_ = p.RecordUndoSnapshot(project, undoAction, ctx, *cfg.UndoHistoryDepth, []string{taskID})
		}
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
		// Auto-set targeted undo for single-task operations
		if len(opts.UndoTaskIDs) == 0 && opts.UndoAction != "" {
			opts.UndoTaskIDs = []string{taskID}
		}
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
	// Run post-mutation hooks so that secondary views (pinned tasks, quick
	// switch, task detail) that use simplePersist still auto-unblock blocked
	// tasks and generate recurring instances — matching the CLI behavior.
	project.CheckAndAutoUnblock()
	project.GenerateRecurringInstances()
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
	UndoTaskIDs   []string // Tasks explicitly changed by user (enables targeted undo)
	UndoGroupID   string
	// UndoGroupProjectIDs identifies every project in a grouped undo
	// transaction. Values must be project IDs, not slugs.
	UndoGroupProjectIDs []string
}

// BuildWorkflowOpts constructs a WorkflowSaveOptions from the given parameters.
// It resolves UndoDepth from the config when undo fields are provided.
func BuildWorkflowOpts(persister Persister, project *models.Project, cfg *config.Config, cfgManager *config.Manager, prevStates map[string]*models.Task, undoAction, undoContext string, undoTaskIDs []string) WorkflowSaveOptions {
	opts := WorkflowSaveOptions{
		Persister:     persister,
		Project:       project,
		Config:        cfg,
		ConfigManager: cfgManager,
		PrevStates:    prevStates,
		UndoAction:    undoAction,
		UndoContext:   undoContext,
		UndoTaskIDs:   undoTaskIDs,
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

	workflowGroupedUndoRecorded := false
	if opts.Config != nil {
		rules := opts.Config.GetEnabledWorkflowRules()
		if len(rules) > 0 {
			var result *workflow.EvaluationResult
			if opts.PrevStates != nil {
				evalCtx := workflow.BuildEvaluationContext(opts.Project.Tasks, opts.PrevStates, opts.Project.Name, opts.Project.ID)
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
				recorded, err := processTUIDeferredMoves(opts.Persister, opts.Project, result.DeferredMoves, opts.Config, opts.ConfigManager, tuiDeferredMoveUndoOptions{
					Action:  opts.UndoAction,
					Context: opts.UndoContext,
					Depth:   opts.UndoDepth,
				})
				if err != nil {
					return fmt.Errorf("workflow move_to_project failed: %w", err)
				}
				workflowGroupedUndoRecorded = workflowGroupedUndoRecorded || recorded
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

	// Re-run post-mutation hooks after workflow rules: rules may have set
	// tasks to "done" (e.g., auto-complete parent), which requires generating
	// recurring instances and unblocking dependents.
	opts.Project.CheckAndAutoUnblock()
	opts.Project.GenerateRecurringInstances()

	opts.Project.ReorderRootTasksByCompletion()

	// Record undo snapshot BEFORE backup+save so it captures the pre-mutation state
	if opts.UndoDepth > 0 && !workflowGroupedUndoRecorded {
		if opts.UndoAction == "" {
			opts.UndoAction = "Task Modified"
		}
		_ = opts.Persister.RecordUndoSnapshotWithOptions(opts.Project, storage.UndoSnapshotOptions{
			Action:          opts.UndoAction,
			Context:         opts.UndoContext,
			MaxDepth:        opts.UndoDepth,
			TaskIDs:         opts.UndoTaskIDs,
			GroupID:         opts.UndoGroupID,
			GroupProjectIDs: opts.UndoGroupProjectIDs,
		})
	}

	if err := opts.Persister.Backup(opts.Project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	if err := opts.Persister.Save(opts.Project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	_ = opts.Persister.Load(opts.Project)
	return nil
}

type tuiDeferredMoveUndoOptions struct {
	Action  string
	Context string
	Depth   int
}

// processTUIDeferredMoves handles move_to_project actions in the TUI context.
// It transfers tasks (and their descendants) from the source project to destination projects.
func processTUIDeferredMoves(p Persister, srcProject *models.Project, moves []workflow.DeferredMove, cfg *config.Config, cfgManager *config.Manager, undoOpts tuiDeferredMoveUndoOptions) (bool, error) {
	if p == nil {
		p = defaultPersister
	}
	if cfg == nil {
		return false, fmt.Errorf("config is required")
	}

	destProjects := make(map[string]*models.Project, len(moves))
	activeDestProjects := make(map[string]*models.Project, len(moves))
	for _, move := range moves {
		destProject, ok := destProjects[move.Destination]
		if !ok {
			destProject = cfg.GetProject(move.Destination)
			if destProject == nil {
				return false, fmt.Errorf("destination project %q not found", move.Destination)
			}
			if err := p.Load(destProject); err != nil {
				return false, fmt.Errorf("failed to load destination project %q: %w", move.Destination, err)
			}
			destProjects[move.Destination] = destProject
		}
		if tuiDeferredMoveTaskExists(srcProject, move.TaskID) {
			activeDestProjects[move.Destination] = destProject
		}
	}

	recordedUndo := false
	if undoOpts.Depth > 0 && len(activeDestProjects) > 0 {
		destinations := make([]string, 0, len(activeDestProjects))
		for destination := range activeDestProjects {
			destinations = append(destinations, destination)
		}
		sort.Strings(destinations)

		projectsToSnapshot := make([]*models.Project, 0, len(destinations)+1)
		projectsToSnapshot = append(projectsToSnapshot, srcProject)
		for _, destination := range destinations {
			projectsToSnapshot = append(projectsToSnapshot, activeDestProjects[destination])
		}

		action := undoOpts.Action
		if action == "" {
			action = "Workflow Move to Project"
		}
		context := undoOpts.Context
		if context == "" {
			context = fmt.Sprintf("%d workflow move(s)", len(moves))
		}
		groupID, groupProjectIDs := undoGroupMetadataForProjects(projectsToSnapshot)
		for _, project := range projectsToSnapshot {
			if err := recordTUIUndoSnapshotWithGroup(p, project, action, context, undoOpts.Depth, nil, groupID, groupProjectIDs); err != nil {
				return false, err
			}
		}
		recordedUndo = true
	}

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
			continue
		}

		// Collect the full subtree (task + all descendants) so children move
		// with their parent, matching the behavior of MoveTaskToProject.
		// Walk by ParentID rather than Children to avoid requiring BuildTaskTree.
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

		// Clear dependencies and related references on moved tasks
		// (they point to tasks in the source project and would become dangling)
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

		// Assign new short IDs in destination and add all moved tasks
		for _, mt := range movedTasks {
			mt.ShortID = destProject.GetNextShortID()
		}
		movedTask.ParentID = nil
		movedTask.Parent = nil
		destProject.Tasks = append(destProject.Tasks, movedTasks...)
		destProject.BuildTaskTree()

		if cfg != nil {
			if updated := cfg.ReassignTaskProject(movedIDs, destProject.ID); updated > 0 && cfgManager != nil {
				if err := cfgManager.Save(cfg); err != nil {
					return false, fmt.Errorf("failed to save moved task references: %w", err)
				}
			}
		}

		if err := p.Backup(destProject); err != nil {
			return false, fmt.Errorf("failed to backup destination project %q: %w", move.Destination, err)
		}
		if err := p.Save(destProject); err != nil {
			return false, fmt.Errorf("failed to save destination project %q: %w", move.Destination, err)
		}
		_ = p.Load(destProject)
	}
	return recordedUndo, nil
}

func tuiDeferredMoveTaskExists(project *models.Project, taskID string) bool {
	if project == nil {
		return false
	}
	for _, task := range project.Tasks {
		if task != nil && task.ID == taskID {
			return true
		}
	}
	return false
}

// reapplySideEffects re-runs state-derived side effects (auto-unblock,
// recurring instances, reorder) after an undo/redo restore. This ensures
// only the user's explicit change is reverted while automated derivations
// are freshly computed. Event-driven workflow rules are deliberately skipped.
func reapplySideEffects(persister Persister, project *models.Project) error {
	project.CheckAndAutoUnblock()
	project.GenerateRecurringInstances()
	project.ReorderRootTasksByCompletion()
	if err := persister.Backup(project); err != nil {
		return fmt.Errorf("failed to backup after undo/redo: %w", err)
	}
	if err := persister.Save(project); err != nil {
		return fmt.Errorf("failed to save after undo/redo: %w", err)
	}
	_ = persister.Load(project)
	return nil
}

// performUndoCmd returns a tea.Cmd that restores the project from the most recent
// undo snapshot. View-level rebuild (buildTree, rebuildRows) is handled by
// handleWithRefresh on the main goroutine when UndoPerformedMsg is received.
func performUndoCmd(persister Persister, project *models.Project) tea.Cmd {
	return performUndoForProjectsCmd(persister, project, nil, nil)
}

type postRestoreHook func([]*models.Project) error

func performUndoForProjectsCmd(persister Persister, project *models.Project, projects []*models.Project, afterRestore postRestoreHook) tea.Cmd {
	if project == nil {
		return nil
	}
	return func() tea.Msg {
		projectsToReload := restoreProjectsForTopTUIHistoryEntry(project, projects, storage.LoadUndoHistory)
		action, err := storage.PerformUndoForProjects(project, projects)
		if err != nil {
			return ErrorMsg{Err: formatUndoRedoRestoreError("undo", err)}
		}
		if err := reloadAndReapplyRestoredProjects(persister, projectsToReload, "undo"); err != nil {
			return ErrorMsg{Err: err}
		}
		if afterRestore != nil {
			if err := afterRestore(projectsToReload); err != nil {
				return ErrorMsg{Err: err}
			}
		}
		return UndoPerformedMsg{Action: action}
	}
}

func reloadAndReapplyRestoredProjects(persister Persister, projects []*models.Project, label string) error {
	for _, project := range uniqueTUIUndoProjects(projects) {
		if err := persister.Load(project); err != nil {
			return fmt.Errorf("failed to reload after %s: %w", label, err)
		}
		if err := reapplySideEffects(persister, project); err != nil {
			return err
		}
	}
	return nil
}

// performRedoCmd returns a tea.Cmd that restores the project from the most recent
// redo snapshot and pushes current state onto the undo stack.
func performRedoCmd(persister Persister, project *models.Project) tea.Cmd {
	return performRedoForProjectsCmd(persister, project, nil, nil)
}

func performRedoForProjectsCmd(persister Persister, project *models.Project, projects []*models.Project, afterRestore postRestoreHook) tea.Cmd {
	if project == nil {
		return nil
	}
	return func() tea.Msg {
		projectsToReload := restoreProjectsForTopTUIHistoryEntry(project, projects, storage.LoadRedoHistory)
		action, err := storage.PerformRedoForProjects(project, projects)
		if err != nil {
			return ErrorMsg{Err: formatUndoRedoRestoreError("redo", err)}
		}
		if err := reloadAndReapplyRestoredProjects(persister, projectsToReload, "redo"); err != nil {
			return ErrorMsg{Err: err}
		}
		if afterRestore != nil {
			if err := afterRestore(projectsToReload); err != nil {
				return ErrorMsg{Err: err}
			}
		}
		return RedoPerformedMsg{Action: action}
	}
}
