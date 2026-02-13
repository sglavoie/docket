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
				evalCtx := buildEvaluationContext(opts.Project.Tasks, opts.PrevStates)
				result = workflow.EvaluateRulesWithContext(evalCtx, rules)
			} else {
				result = workflow.EvaluateRules(opts.Project.Tasks, rules)
			}
			if len(result.Conflicts) > 0 {
				_ = result.Conflicts
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

// applyDeleteTask removes a task from a project.
func applyDeleteTask(pf persistFn, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	shortID := task.ShortID

	// Remove the task from the project first, then trigger the persist
	// pipeline via a no-op mutate on any remaining task.
	for i, t := range project.Tasks {
		if t.ID == taskID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

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

// deleteTasksFromProject removes the given task IDs (and all their descendants)
// from the project. It handles attachment cleanup, dangling parent references,
// and returns the filtered task list. If focusedTaskID points to a deleted task,
// it is set to nil.
func deleteTasksFromProject(project *models.Project, persister Persister, targetIDs map[string]bool, focusedTaskID **models.Task) error {
	idsToDelete := collectDescendantIDs(project.Tasks, targetIDs)

	// Clean up attachments for all tasks being deleted
	for _, t := range project.Tasks {
		if idsToDelete[t.ID] {
			_ = storage.CleanupTaskAttachments(project, t)
		}
	}

	// Exit focus mode if focused task is being deleted
	if focusedTaskID != nil && *focusedTaskID != nil && idsToDelete[(*focusedTaskID).ID] {
		*focusedTaskID = nil
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

	if err := persister.Save(project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}
	_ = persister.Load(project)
	return nil
}

// SecondaryViewMovePrefixHints contains "m" prefix hints for secondary views.
var SecondaryViewMovePrefixHints = []PrefixHint{
	{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
	{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
	{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
	{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
	{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
	{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
}
