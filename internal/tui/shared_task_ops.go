package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TaskOpResult holds the outcome of a shared task operation.
type TaskOpResult struct {
	StatusMessage string
	Err           error
	Cmd           tea.Cmd
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

// applyStatusChange validates and applies a status transition on the given task.
// It handles backup, mutation, save, and reload. The caller is responsible for
// refreshing its view-specific data (loadTasks, clampCursor, etc.).
func applyStatusChange(p Persister, project *models.Project, taskID string, newStatus models.TaskStatus) TaskOpResult {
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

	result := withPersist(p, project, taskID, func(t *models.Task) {
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
func applyPriorityChange(p Persister, project *models.Project, taskID string, priority int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if task.Priority == priority {
		return TaskOpResult{}
	}

	result := withPersist(p, project, taskID, func(t *models.Task) {
		t.Priority = priority
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Priority set to %d", priority)
	return result
}

// applyQuickDueDate sets the due date relative to today.
func applyQuickDueDate(p Persister, project *models.Project, taskID string, daysFromNow int) TaskOpResult {
	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = util.StartOfDay(newDate)

	result := withPersist(p, project, taskID, func(t *models.Task) {
		t.DueDate = &newDate
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Due date set to %s", dueDateLabel(daysFromNow))
	return result
}

// applyClearDueDate removes the due date from a task.
func applyClearDueDate(p Persister, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}
	if task.DueDate == nil {
		return TaskOpResult{StatusMessage: "Task has no due date"}
	}

	result := withPersist(p, project, taskID, func(t *models.Task) {
		t.DueDate = nil
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = "Due date cleared"
	return result
}

// applyToggleActive toggles the Active flag on a task.
func applyToggleActive(p Persister, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	wasActive := task.Active
	result := withPersist(p, project, taskID, func(t *models.Task) {
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
	result.Cmd = func() tea.Msg { return TaskUpdatedMsg{Task: task} }
	return result
}

// applyRemoveFromFocus always sets Active = false (used by FocusView specifically).
func applyRemoveFromFocus(p Persister, project *models.Project, taskID string, shortID int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := withPersist(p, project, taskID, func(t *models.Task) {
		t.Active = false
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	result.StatusMessage = fmt.Sprintf("Removed #%d from focus", shortID)
	result.Cmd = func() tea.Msg { return TaskUpdatedMsg{Task: task} }
	return result
}

// applySectionChange sets the GTD section on a task.
func applySectionChange(p Persister, project *models.Project, taskID string, section string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	result := withPersist(p, project, taskID, func(t *models.Task) {
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
	result.Cmd = func() tea.Msg { return TaskUpdatedMsg{Task: task} }
	return result
}

// applyDeleteTask removes a task from a project.
func applyDeleteTask(p Persister, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	shortID := task.ShortID

	if err := p.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	for i, t := range project.Tasks {
		if t.ID == taskID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	if err := p.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = p.Load(project)

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Deleted task #%d", shortID),
		Cmd: func() tea.Msg {
			return TaskDeletedMsg{Task: task}
		},
	}
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

// SecondaryViewMovePrefixHints contains "m" prefix hints for secondary views.
var SecondaryViewMovePrefixHints = []PrefixHint{
	{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
	{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
	{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
	{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
	{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
	{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
}
