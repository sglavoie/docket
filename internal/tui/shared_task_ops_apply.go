package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// analyzeBulkStatusTransition separates tasks that can transition to newStatus
// from those blocked by transition rules or incomplete dependencies.
func analyzeBulkStatusTransition(project *models.Project, tasks []*models.Task, newStatus models.TaskStatus) (valid []*models.Task, blockedCount int) {
	for _, task := range tasks {
		if task.CanTransitionTo(newStatus) != "" {
			continue
		}
		if newStatus == models.StatusDone && project.HasIncompleteDependencies(task) {
			blockedCount++
			continue
		}
		valid = append(valid, task)
	}
	return
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

	if newStatus == models.StatusDone {
		var incompleteChildren []*models.Task
		for _, child := range task.Children {
			if child.Status != models.StatusDone && child.Status != models.StatusCanceled {
				incompleteChildren = append(incompleteChildren, child)
			}
		}
		if len(incompleteChildren) > 0 {
			var msg strings.Builder
			fmt.Fprintf(&msg, "cannot complete #%d: has %d incomplete children:", task.ShortID, len(incompleteChildren))
			for _, child := range incompleteChildren {
				fmt.Fprintf(&msg, " #%d", child.ShortID)
			}
			return TaskOpResult{StatusMessage: msg.String()}
		}
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

// applyStageChange sets the workflow stage on a task.
func applyStageChange(pf persistFn, project *models.Project, taskID string, stage string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if task.Stage == stage {
		return TaskOpResult{}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.Stage = stage
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	if stage == "" {
		result.StatusMessage = "Stage cleared"
	} else {
		result.StatusMessage = fmt.Sprintf("Stage set to %s", stage)
	}
	return result
}

// applyQuickDueDate sets the due date relative to today.
func applyQuickDueDate(pf persistFn, project *models.Project, taskID string, daysFromNow int) TaskOpResult {
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = util.StartOfDay(newDate)

	result := pf(project, taskID, func(t *models.Task) {
		t.DueDate = &newDate
		t.HasTime = false // Quick presets are always date-only.
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
		t.HasTime = false
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

// applyQuickSnooze sets the snooze date on a task.
func applyQuickSnooze(pf persistFn, project *models.Project, taskID string, snoozeDate *time.Time) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	if snoozeDate == nil {
		result.StatusMessage = "Snooze cleared"
	} else {
		result.StatusMessage = fmt.Sprintf("Snoozed until %s", snoozeDate.Format("Mon Jan 2"))
	}
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
		t.ToggleActive()
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
// The persister is used to save the project directly when all tasks are deleted
// (the persistFn requires a task ID, which is unavailable when the list is empty).
func applyDeleteTask(pf persistFn, persister Persister, project *models.Project, taskID string) TaskOpResult {
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
			if err := storage.CleanupTaskAttachments(project, t); err != nil {
				logging.Warn("failed to clean up attachments for task %s: %v", t.ID, err)
			}
		}
	}

	// Filter out deleted tasks and clear dangling references
	var remaining []*models.Task
	for _, t := range project.Tasks {
		if !idsToDelete[t.ID] {
			if t.ParentID != nil && idsToDelete[*t.ParentID] {
				t.ParentID = nil
			}
			// Clear dangling DependsOn references to deleted tasks
			if len(t.DependsOn) > 0 {
				cleaned := t.DependsOn[:0]
				for _, depID := range t.DependsOn {
					if !idsToDelete[depID] {
						cleaned = append(cleaned, depID)
					}
				}
				t.DependsOn = cleaned
			}
			// Clear dangling RelatedTo references to deleted tasks
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
	}
	project.Tasks = remaining

	// Trigger the persist pipeline via a no-op mutate on any remaining task.
	// When all tasks have been deleted, save directly via the persister since
	// persistFn requires a valid task ID.
	if len(project.Tasks) > 0 {
		result := pf(project, project.Tasks[0].ID, func(t *models.Task) {})
		if result.Err != nil {
			return result
		}
	} else {
		if err := persister.SaveWithBackupSync(project); err != nil {
			return TaskOpResult{Err: fmt.Errorf("failed to save empty project: %w", err)}
		}
	}

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Deleted task #%d", shortID),
		Cmd: func() tea.Msg {
			return TaskDeletedMsg{Task: task}
		},
	}
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

// applyToggleDimmed toggles the Dimmed flag on a task.
func applyToggleDimmed(pf persistFn, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := pf(project, taskID, func(t *models.Task) {
		t.ToggleDimmed()
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	// Re-read to get updated state
	updated := project.GetTaskByID(taskID)
	if updated != nil {
		if updated.Dimmed {
			result.StatusMessage = fmt.Sprintf("Dimmed task #%d", updated.ShortID)
		} else {
			result.StatusMessage = fmt.Sprintf("Undimmed task #%d", updated.ShortID)
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

// dueDateFromDays returns a *time.Time pointer and description for a relative
// day offset. When isClear is true, it returns (nil, "cleared").
// The returned time is normalized to start-of-day (midnight) for consistency
// with all other due-date setters and the day-level storage format.
func dueDateFromDays(days int, isClear bool) (*time.Time, string) {
	if isClear {
		return nil, "cleared"
	}
	d := util.StartOfDay(time.Now().AddDate(0, 0, days))
	return &d, dueDateLabel(days)
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
