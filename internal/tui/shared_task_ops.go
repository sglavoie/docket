package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
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
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())

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

// SecondaryViewMovePrefixHints contains "m" prefix hints for secondary views.
var SecondaryViewMovePrefixHints = []PrefixHint{
	{Key: "a", Description: "→Active", Detail: "Send task to Active section"},
	{Key: "n", Description: "→Next", Detail: "Send task to Next section"},
	{Key: "i", Description: "→Inbox", Detail: "Send task to Inbox section"},
	{Key: "w", Description: "→Waiting", Detail: "Send task to Waiting section"},
	{Key: "s", Description: "→Someday", Detail: "Send task to Someday section"},
	{Key: "r", Description: "→Review", Detail: "Send task to Review section"},
}
