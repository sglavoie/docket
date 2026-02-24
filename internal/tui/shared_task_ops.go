package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
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
// ProjectTree, Kanban, and Outliner all satisfy this interface.
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

// analyzeBulkStatusTransition separates tasks that can transition to newStatus
// from those blocked by transition rules or incomplete dependencies.
func analyzeBulkStatusTransition(project *models.Project, tasks []*models.Task, newStatus models.TaskStatus) (valid []*models.Task, blockedCount int) {
	for _, task := range tasks {
		if task.CanTransitionTo(newStatus) != "" {
			continue
		}
		if newStatus == models.StatusDone && len(project.GetIncompleteDependencies(task)) > 0 {
			blockedCount++
			continue
		}
		valid = append(valid, task)
	}
	return
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

// persistWithWorkflow runs the full save pipeline used by both ProjectTree and Kanban:
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
				_ = opts.ConfigManager.Save(opts.Config)
			}
		}
	}

	opts.Project.ReorderRootTasksByCompletion()

	if opts.UndoDepth > 0 && opts.UndoAction != "" {
		_ = opts.Persister.RecordUndoSnapshot(opts.Project, opts.UndoAction, opts.UndoContext, opts.UndoDepth)
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

// processTUIDeferredMoves handles move_to_project actions in the TUI context.
// It transfers tasks from the source project to destination projects.
func processTUIDeferredMoves(p Persister, srcProject *models.Project, moves []workflow.DeferredMove, cfg *config.Config) error {
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

// applyStatusChange validates and applies a status transition on the given task.
// It handles backup, mutation, save, and reload. The caller is responsible for
// refreshing its view-specific data (loadTasks, clampCursor, etc.).
func applyStatusChange(pf persistFn, project *models.Project, taskID string, newStatus models.TaskStatus) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if msg := task.CanTransitionTo(newStatus); msg != "" {
		return TaskOpResult{StatusMessage: msg}
	}

	if newStatus == models.StatusDone && project.HasIncompleteDependencies(task) {
		return TaskOpResult{Err: fmt.Errorf("cannot complete: has incomplete dependencies")}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.ApplyTransition(newStatus)
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus)
	result.Cmd = func() tea.Msg { return TaskStatusChangedMsg{Task: task} }
	return result
}

// applyPriorityChange sets the priority on a task.
func applyPriorityChange(pf persistFn, project *models.Project, taskID string, priority int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if task.Priority == priority {
		return TaskOpResult{}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.Priority = priority
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Priority set to %d", priority)
	return result
}

// applyQuickDueDate sets the due date relative to today.
func applyQuickDueDate(pf persistFn, project *models.Project, taskID string, daysFromNow int) TaskOpResult {
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = util.StartOfDay(newDate)

	result := pf(project, taskID, func(t *models.Task) {
		t.DueDate = &newDate
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Due date set to %s", dueDateLabel(daysFromNow))
	return result
}

// applyClearDueDate removes the due date from a task.
func applyClearDueDate(pf persistFn, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if task.DueDate == nil {
		return TaskOpResult{StatusMessage: "Task has no due date"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.DueDate = nil
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = "Due date cleared"
	return result
}

// resolveQuickSnoozeDate maps a snooze sub-key to a snooze date and description.
// Returns (nil, "", false) for unrecognized keys. The "d" (date picker) case
// must be handled by the caller.
func resolveQuickSnoozeDate(key string) (snoozeDate *time.Time, desc string, ok bool) {
	today := util.StartOfDay(time.Now())

	switch key {
	case "t":
		d := today.AddDate(0, 0, 1)
		return &d, "tomorrow", true
	case "w":
		d := today.AddDate(0, 0, 7)
		return &d, "+1 week", true
	case "m":
		d := today.AddDate(0, 1, 0)
		return &d, "+1 month", true
	case "n":
		daysUntilMon := (8 - int(today.Weekday())) % 7
		if daysUntilMon == 0 {
			daysUntilMon = 7
		}
		d := today.AddDate(0, 0, daysUntilMon)
		return &d, "next Monday", true
	case "0":
		return nil, "cleared", true
	default:
		return nil, "", false
	}
}

// applyToggleActive toggles the Active flag on a task.
func applyToggleActive(pf persistFn, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	wasActive := task.Active
	result := pf(project, taskID, func(t *models.Task) {
		t.Active = !t.Active
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	msg := fmt.Sprintf("Task #%d marked active", task.ShortID)
	if wasActive {
		msg = fmt.Sprintf("Task #%d unmarked active", task.ShortID)
	}
	result.StatusMessage = msg
	result.Cmd = func() tea.Msg { return TaskPropertyChangedMsg{Task: task} }
	return result
}

// applyRemoveFromFocus always sets Active = false (used by FocusView specifically).
func applyRemoveFromFocus(pf persistFn, project *models.Project, taskID string, shortID int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.Active = false
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Removed #%d from focus", shortID)
	result.Cmd = func() tea.Msg { return TaskPropertyChangedMsg{Task: task} }
	return result
}

// applySectionChange sets the GTD section on a task.
func applySectionChange(pf persistFn, project *models.Project, taskID string, section string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		assignSection(t, section)
		if section != "" && section != removeCustomSectionSentinel {
			project.EnsureSection(section)
		}
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	sectionName := section
	if sectionName == "" {
		sectionName = "Active"
	}
	result.StatusMessage = fmt.Sprintf("Moved to %s", sectionName)
	result.Cmd = func() tea.Msg { return TaskPropertyChangedMsg{Task: task} }
	return result
}

// applyDeleteTask removes a task and its descendants from a project.
// It handles attachment cleanup, descendant collection, and dangling parent references.
func applyDeleteTask(pf persistFn, project *models.Project, taskID string, dataDir string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	shortID := task.ShortID

	// Collect the task and all its descendants
	idsToDelete := collectDescendantIDs(project.Tasks, map[string]bool{taskID: true})

	// Clean up attachments for all tasks being deleted
	for _, t := range project.Tasks {
		if idsToDelete[t.ID] {
			_ = storage.CleanupTaskAttachments(project, t)
		}
	}

	// Filter out deleted tasks and clear dangling parent references
	var remaining []*models.Task
	for _, t := range project.Tasks {
		if !idsToDelete[t.ID] {
			if t.ParentID != nil && idsToDelete[*t.ParentID] {
				t.ParentID = nil
			}
			remaining = append(remaining, t)
		}
	}
	project.Tasks = remaining

	// Trigger the persist pipeline via a no-op mutate on any remaining task.
	if len(project.Tasks) > 0 {
		result := pf(project, project.Tasks[0].ID, func(t *models.Task) {})
		if result.Err != nil {
			return result
		}
	}

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Deleted task #%d", shortID),
		Cmd: func() tea.Msg {
			return TaskDeletedMsg{Task: task}
		},
	}
}

// TypeChangeOp represents the kind of type change operation.
type TypeChangeOp int

const (
	TypePromote    TypeChangeOp = iota // Promote type (task → story → epic)
	TypeDemote                         // Demote type (epic → story → task)
	TypeCycleLeft                      // Cycle leaf type left
	TypeCycleRight                     // Cycle leaf type right
)

// applyTypeChange validates and applies a type change (promote, demote, cycle)
// on the given task. It handles the can-do guard, persist, and status message.
func applyTypeChange(pf persistFn, project *models.Project, taskID string, op TypeChangeOp) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	var canDo bool
	switch op {
	case TypePromote:
		canDo, _ = task.CanPromote()
	case TypeDemote:
		canDo, _ = task.CanDemote()
	case TypeCycleLeft:
		canDo, _ = task.CanCycleLeafType(true)
	case TypeCycleRight:
		canDo, _ = task.CanCycleLeafType(false)
	}
	if !canDo {
		return TaskOpResult{}
	}

	result := pf(project, taskID, func(t *models.Task) {
		switch op {
		case TypePromote:
			_ = t.Promote()
		case TypeDemote:
			_ = t.Demote()
		case TypeCycleLeft:
			_ = t.CycleLeafType(true)
		case TypeCycleRight:
			_ = t.CycleLeafType(false)
		}
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	// Re-read the task after persist to get the updated type
	updated := project.GetTaskByID(taskID)
	if updated != nil {
		result.StatusMessage = fmt.Sprintf("Changed to %s", updated.Type.HeadingPrefix())
	}
	return result
}

// applyTypeChangeOp applies a type change operation to a task in-place without
// persisting. Returns true if the change was applied, false if the task was
// skipped (not found or constraint violation). Used by bulk type change.
func applyTypeChangeOp(project *models.Project, taskID string, op TypeChangeOp) bool {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return false
	}

	var canDo bool
	switch op {
	case TypePromote:
		canDo, _ = task.CanPromote()
	case TypeDemote:
		canDo, _ = task.CanDemote()
	case TypeCycleLeft:
		canDo, _ = task.CanCycleLeafType(true)
	case TypeCycleRight:
		canDo, _ = task.CanCycleLeafType(false)
	}
	if !canDo {
		return false
	}

	project.MutateTask(taskID, func(t *models.Task) {
		switch op {
		case TypePromote:
			_ = t.Promote()
		case TypeDemote:
			_ = t.Demote()
		case TypeCycleLeft:
			_ = t.CycleLeafType(true)
		case TypeCycleRight:
			_ = t.CycleLeafType(false)
		}
	})
	return true
}

// applyBulkTypeCycle sets all selected tasks to the same leaf type, then cycles
// them together. If the tasks already share a common type, that type is cycled.
// If types are mixed, all tasks are first normalized to Task before cycling.
// Non-leaf tasks and tasks that fail hierarchy validation are skipped.
func applyBulkTypeCycle(project *models.Project, tasks []*models.Task, op TypeChangeOp) int {
	// Determine whether all tasks share the same leaf type.
	var commonType models.TaskType
	uniform := true
	first := true
	for _, task := range tasks {
		t := project.GetTaskByID(task.ID)
		if t == nil || !t.Type.IsLeafType() {
			continue
		}
		if first {
			commonType = t.Type
			first = false
		} else if t.Type != commonType {
			uniform = false
			break
		}
	}

	// Compute the target type: cycle from common type, or reset to Task then cycle.
	var targetType models.TaskType
	if uniform && !first {
		// All same leaf type — cycle from it.
		if op == TypeCycleLeft {
			targetType = commonType.CycleLeafLeft()
		} else {
			targetType = commonType.CycleLeafRight()
		}
	} else {
		// Mixed types — normalize to Task first, then cycle.
		if op == TypeCycleLeft {
			targetType = models.TaskTypeTask.CycleLeafLeft()
		} else {
			targetType = models.TaskTypeTask.CycleLeafRight()
		}
	}

	// Apply the target type to all eligible tasks.
	changed := 0
	for _, task := range tasks {
		t := project.GetTaskByID(task.ID)
		if t == nil || !t.Type.IsLeafType() {
			continue
		}
		if t.Type == targetType {
			continue
		}
		// Validate hierarchy: new type must be valid under parent.
		if t.Parent != nil && !targetType.CanBeChildOf(t.Parent.Type) {
			continue
		}
		project.MutateTask(t.ID, func(mt *models.Task) {
			mt.Type = targetType
			mt.ClearTypeAttrs()
		})
		changed++
	}
	return changed
}

// applyTogglePinned toggles the Pinned flag on a task.
func applyTogglePinned(pf persistFn, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.TogglePinned()
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	// Re-read to get updated state
	updated := project.GetTaskByID(taskID)
	if updated != nil {
		if updated.Pinned {
			result.StatusMessage = fmt.Sprintf("Pinned task #%d", updated.ShortID)
		} else {
			result.StatusMessage = fmt.Sprintf("Unpinned task #%d", updated.ShortID)
		}
	}
	result.Cmd = func() tea.Msg { return TaskPropertyChangedMsg{Task: task} }
	return result
}

// IndentTask attempts to indent a task under a valid parent by walking up from
// the candidate (the task visually above). It sets ParentID and rebuilds the tree.
// Returns true if the indent was performed.
func IndentTask(project *models.Project, task *models.Task, candidate *models.Task) bool {
	for candidate != nil {
		if ok, _ := task.CanIndent(candidate); ok {
			candidateID := candidate.ID
			task.ParentID = &candidateID
			project.BuildTaskTree()
			return true
		}
		candidate = candidate.Parent
	}
	return false
}

// OutdentTask attempts to outdent a task to its parent's level.
// Returns true if the outdent was performed.
func OutdentTask(project *models.Project, task *models.Task) bool {
	if ok, _ := task.CanOutdent(); !ok {
		return false
	}
	_ = project.OutdentTask(task)
	return true
}

// dueDateLabel returns a human-friendly label for a relative due date.
func dueDateLabel(daysFromNow int) string {
	switch daysFromNow {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	case 7:
		return "next week"
	case 30:
		return "next month"
	default:
		return fmt.Sprintf("+%d days", daysFromNow)
	}
}

// viewModeForKey maps a `g` prefix second key to a ProjectTree ViewMode.
// Returns the view mode and true if valid, or zero value and false if not.
func viewModeForKey(key string) (ViewMode, bool) {
	switch key {
	case "a":
		return ViewModeActive, true
	case "n":
		return ViewModeNext, true
	case "i":
		return ViewModeInbox, true
	case "w":
		return ViewModeWaiting, true
	case "s":
		return ViewModeSomeday, true
	case "z":
		return ViewModeSnoozed, true
	case "*":
		return ViewModeAll, true
	case "r":
		return ViewModeReview, true
	case "S":
		return ViewModeStandby, true
	case "c":
		return ViewModeCompleted, true
	}
	return 0, false
}

// sectionForMoveKey maps a move prefix second key to a GTD section name.
// Returns the section name and true if valid, or empty string and false if not.
func sectionForMoveKey(key string) (string, bool) {
	switch key {
	case "a":
		return "", true // Active = empty section
	case "n":
		return models.SectionNext, true
	case "i":
		return models.SectionInbox, true
	case "w":
		return models.SectionWaiting, true
	case "s":
		return models.SectionSomeday, true
	case "r":
		return models.SectionReview, true
	case "S":
		return models.SectionStandby, true
	}
	return "", false
}

// StatusForPrefixKey maps a prefix second-key to a TaskStatus.
// Returns the status and true if the key is valid, or zero value and false otherwise.
func StatusForPrefixKey(key string) (models.TaskStatus, bool) {
	switch key {
	case "s":
		return models.StatusInProgress, true
	case "d":
		return models.StatusDone, true
	case "b":
		return models.StatusBlocked, true
	case "o":
		return models.StatusTodo, true
	case "x":
		return models.StatusCanceled, true
	case "r":
		return models.StatusReview, true
	case "S":
		return models.StatusStandby, true
	}
	return "", false
}

// StatusVerb returns the past-tense verb for a status transition.
func StatusVerb(status models.TaskStatus) string {
	switch status {
	case models.StatusInProgress:
		return "started"
	case models.StatusDone:
		return "completed"
	case models.StatusBlocked:
		return "blocked"
	case models.StatusTodo:
		return "reopened"
	case models.StatusCanceled:
		return "canceled"
	case models.StatusReview:
		return "sent to review"
	case models.StatusStandby:
		return "put on standby"
	}
	return string(status)
}

// DueDateDaysForPrefixKey maps a due-date prefix second-key to a day offset.
// Returns (days, isClear, matched). When isClear is true the caller should
// remove the due date instead of setting one.
func DueDateDaysForPrefixKey(key string) (int, bool, bool) {
	switch key {
	case "t":
		return 0, false, true // today
	case "m":
		return 1, false, true // tomorrow
	case "w":
		return 7, false, true // next week
	case "n":
		return 30, false, true // next month
	case "0":
		return 0, true, true // clear
	}
	return 0, false, false
}

// PriorityForPrefixKey maps a priority prefix second-key to a priority value.
// Returns the priority (1-5) and true if valid.
func PriorityForPrefixKey(key string) (int, bool) {
	if len(key) == 1 && key[0] >= '1' && key[0] <= '5' {
		return int(key[0] - '0'), true
	}
	return 0, false
}

// dueDateFromDays returns a *time.Time pointer and description for a relative
// day offset. When isClear is true, it returns (nil, "cleared").
func dueDateFromDays(days int, isClear bool) (*time.Time, string) {
	if isClear {
		return nil, "cleared"
	}
	now := time.Now()
	d := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 0, now.Location())
	return &d, dueDateLabel(days)
}

// applyBulkPriority sets the priority on multiple tasks. Returns the count of
// tasks actually changed.
func applyBulkPriority(project *models.Project, tasks []*models.Task, priority int) int {
	count := 0
	for _, task := range tasks {
		if project.MutateTask(task.ID, func(t *models.Task) {
			t.Priority = priority
		}) {
			count++
		}
	}
	return count
}

// applyBulkDueDate sets (or clears) the due date on multiple tasks.
// Returns the count of tasks changed.
func applyBulkDueDate(project *models.Project, tasks []*models.Task, dueDate *time.Time) int {
	count := 0
	for _, task := range tasks {
		if project.MutateTask(task.ID, func(t *models.Task) {
			t.DueDate = dueDate
		}) {
			count++
		}
	}
	return count
}

// collectDescendantIDs collects all IDs of a task and its descendants recursively.
func collectDescendantIDs(tasks []*models.Task, rootIDs map[string]bool) map[string]bool {
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

// SecondaryViewMovePrefixHints contains "m" prefix hints for secondary views.
var SecondaryViewMovePrefixHints = []PrefixHint{
	{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
	{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
	{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
	{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
	{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
	{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
	{Key: "S", Description: "→Standby", Detail: "Send task to Standby section"},
}
