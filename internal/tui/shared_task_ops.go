package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TaskOpResult holds the outcome of a shared task operation.
type TaskOpResult struct {
	StatusMessage string
	Err           error
	Cmd           tea.Cmd
}

// applyStatusChange validates and applies a status transition on the given task.
// It handles backup, mutation, save, and reload. The caller is responsible for
// refreshing its view-specific data (loadTasks, clampCursor, etc.).
func applyStatusChange(project *models.Project, taskID string, newStatus models.TaskStatus) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	// Skip completed tasks for most status changes
	if task.Status.IsComplete() && newStatus != models.StatusTodo {
		return TaskOpResult{StatusMessage: "Cannot change status of completed task"}
	}

	// Validate status transition
	switch newStatus {
	case models.StatusInProgress:
		if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
			return TaskOpResult{StatusMessage: "Task must be todo or blocked to start"}
		}
	case models.StatusDone:
		if !task.Status.IsActive() {
			return TaskOpResult{StatusMessage: "Task is not active"}
		}
		if project.HasIncompleteDependencies(task) {
			return TaskOpResult{Err: fmt.Errorf("cannot complete: has incomplete dependencies")}
		}
	case models.StatusBlocked:
		if !task.Status.IsActive() || task.Status == models.StatusBlocked {
			return TaskOpResult{StatusMessage: "Task must be active and not already blocked"}
		}
	case models.StatusCanceled:
		if !task.Status.IsActive() {
			return TaskOpResult{StatusMessage: "Task is not active"}
		}
	case models.StatusTodo:
		if task.Status == models.StatusTodo {
			return TaskOpResult{StatusMessage: "Task is already todo"}
		}
	case models.StatusReview:
		if !task.Status.IsActive() || task.Status == models.StatusReview {
			return TaskOpResult{StatusMessage: "Task must be active and not already in review"}
		}
	}

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.Status = newStatus
		if newStatus.IsComplete() {
			now := time.Now()
			t.CompletedAt = &now
			t.Active = false
		} else {
			t.CompletedAt = nil
		}
	})

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus),
		Cmd: func() tea.Msg {
			return TaskStatusChangedMsg{Task: task}
		},
	}
}

// applyStatusChangePS is like applyStatusChange but uses a ProjectService.
func applyStatusChangePS(ps *ProjectService, project *models.Project, taskID string, newStatus models.TaskStatus) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if task.Status.IsComplete() && newStatus != models.StatusTodo {
		return TaskOpResult{StatusMessage: "Cannot change status of completed task"}
	}

	switch newStatus {
	case models.StatusInProgress:
		if task.Status != models.StatusTodo && task.Status != models.StatusBlocked {
			return TaskOpResult{StatusMessage: "Task must be todo or blocked to start"}
		}
	case models.StatusDone:
		if !task.Status.IsActive() {
			return TaskOpResult{StatusMessage: "Task is not active"}
		}
		if project.HasIncompleteDependencies(task) {
			return TaskOpResult{Err: fmt.Errorf("cannot complete: has incomplete dependencies")}
		}
	case models.StatusBlocked:
		if !task.Status.IsActive() || task.Status == models.StatusBlocked {
			return TaskOpResult{StatusMessage: "Task must be active and not already blocked"}
		}
	case models.StatusCanceled:
		if !task.Status.IsActive() {
			return TaskOpResult{StatusMessage: "Task is not active"}
		}
	case models.StatusTodo:
		if task.Status == models.StatusTodo {
			return TaskOpResult{StatusMessage: "Task is already todo"}
		}
	case models.StatusReview:
		if !task.Status.IsActive() || task.Status == models.StatusReview {
			return TaskOpResult{StatusMessage: "Task must be active and not already in review"}
		}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.Status = newStatus
		if newStatus.IsComplete() {
			now := time.Now()
			t.CompletedAt = &now
			t.Active = false
		} else {
			t.CompletedAt = nil
		}
	})

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Task #%d -> %s", task.ShortID, newStatus),
		Cmd: func() tea.Msg {
			return TaskStatusChangedMsg{Task: task}
		},
	}
}

// applyPriorityChange sets the priority on a task.
func applyPriorityChange(project *models.Project, taskID string, priority int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if task.Priority == priority {
		return TaskOpResult{}
	}

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.Priority = priority
	})

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	return TaskOpResult{StatusMessage: fmt.Sprintf("Priority set to %d", priority)}
}

// applyPriorityChangePS is like applyPriorityChange but uses a ProjectService.
func applyPriorityChangePS(ps *ProjectService, project *models.Project, taskID string, priority int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if task.Priority == priority {
		return TaskOpResult{}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.Priority = priority
	})

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	return TaskOpResult{StatusMessage: fmt.Sprintf("Priority set to %d", priority)}
}

// applyQuickDueDate sets the due date relative to today.
func applyQuickDueDate(project *models.Project, taskID string, daysFromNow int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	project.MutateTask(taskID, func(t *models.Task) {
		t.DueDate = &newDate
	})

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	return TaskOpResult{StatusMessage: fmt.Sprintf("Due date set to %s", dueDateLabel(daysFromNow))}
}

// applyQuickDueDatePS is like applyQuickDueDate but uses a ProjectService.
func applyQuickDueDatePS(ps *ProjectService, project *models.Project, taskID string, daysFromNow int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	newDate := time.Now().AddDate(0, 0, daysFromNow)
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	project.MutateTask(taskID, func(t *models.Task) {
		t.DueDate = &newDate
	})

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	return TaskOpResult{StatusMessage: fmt.Sprintf("Due date set to %s", dueDateLabel(daysFromNow))}
}

// applyClearDueDate removes the due date from a task.
func applyClearDueDate(project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if task.DueDate == nil {
		return TaskOpResult{StatusMessage: "Task has no due date"}
	}

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.DueDate = nil
	})

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	return TaskOpResult{StatusMessage: "Due date cleared"}
}

// applyClearDueDatePS is like applyClearDueDate but uses a ProjectService.
func applyClearDueDatePS(ps *ProjectService, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if task.DueDate == nil {
		return TaskOpResult{StatusMessage: "Task has no due date"}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.DueDate = nil
	})

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	return TaskOpResult{StatusMessage: "Due date cleared"}
}

// applyToggleActive toggles the Active flag on a task.
func applyToggleActive(project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	wasActive := task.Active
	project.MutateTask(taskID, func(t *models.Task) {
		t.Active = !t.Active
	})

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	msg := fmt.Sprintf("Task #%d marked active", task.ShortID)
	if wasActive {
		msg = fmt.Sprintf("Task #%d unmarked active", task.ShortID)
	}

	return TaskOpResult{
		StatusMessage: msg,
		Cmd: func() tea.Msg {
			return TaskUpdatedMsg{Task: task}
		},
	}
}

// applyToggleActivePS is like applyToggleActive but uses a ProjectService.
func applyToggleActivePS(ps *ProjectService, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	wasActive := task.Active
	project.MutateTask(taskID, func(t *models.Task) {
		t.Active = !t.Active
	})

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	msg := fmt.Sprintf("Task #%d marked active", task.ShortID)
	if wasActive {
		msg = fmt.Sprintf("Task #%d unmarked active", task.ShortID)
	}

	return TaskOpResult{
		StatusMessage: msg,
		Cmd: func() tea.Msg {
			return TaskUpdatedMsg{Task: task}
		},
	}
}

// applyRemoveFromFocus always sets Active = false (used by FocusView specifically).
func applyRemoveFromFocusPS(ps *ProjectService, project *models.Project, taskID string, shortID int) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		t.Active = false
	})

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Removed #%d from focus", shortID),
		Cmd: func() tea.Msg {
			return TaskUpdatedMsg{Task: task}
		},
	}
}

// applySectionChange sets the GTD section on a task.
func applySectionChange(project *models.Project, taskID string, section string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		assignSection(t, section)
	})

	if section != "" && section != removeCustomSectionSentinel {
		project.EnsureSection(section)
	}

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	sectionName := section
	if sectionName == "" {
		sectionName = "Active"
	}

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Moved to %s", sectionName),
		Cmd: func() tea.Msg {
			return TaskUpdatedMsg{Task: task}
		},
	}
}

// applySectionChangePS is like applySectionChange but uses a ProjectService.
func applySectionChangePS(ps *ProjectService, project *models.Project, taskID string, section string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	project.MutateTask(taskID, func(t *models.Task) {
		assignSection(t, section)
	})

	if section != "" && section != removeCustomSectionSentinel {
		project.EnsureSection(section)
	}

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

	sectionName := section
	if sectionName == "" {
		sectionName = "Active"
	}

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Moved to %s", sectionName),
		Cmd: func() tea.Msg {
			return TaskUpdatedMsg{Task: task}
		},
	}
}

// applyDeleteTask removes a task from a project.
func applyDeleteTask(project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	shortID := task.ShortID

	if err := storage.BackupProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	for i, t := range project.Tasks {
		if t.ID == taskID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	if err := storage.SaveProject(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = storage.LoadProject(project)

	return TaskOpResult{
		StatusMessage: fmt.Sprintf("Deleted task #%d", shortID),
		Cmd: func() tea.Msg {
			return TaskDeletedMsg{Task: task}
		},
	}
}

// applyDeleteTaskPS is like applyDeleteTask but uses a ProjectService.
func applyDeleteTaskPS(ps *ProjectService, project *models.Project, taskID string) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	shortID := task.ShortID

	if err := ps.Backup(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("backup failed: %w", err)}
	}

	for i, t := range project.Tasks {
		if t.ID == taskID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	if err := ps.Save(project); err != nil {
		return TaskOpResult{Err: fmt.Errorf("save failed: %w", err)}
	}

	_ = ps.Load(project)

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
