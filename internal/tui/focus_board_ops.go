package tui

import (
	"fmt"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// markDone marks the current task as done.
func (m *FocusBoardModel) markDone() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	task := row.Task
	project := row.Project

	if msg := task.CanTransitionTo(models.StatusDone); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusDone)
	})
	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Marked #%d done", task.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// dismiss cancels the current task.
func (m *FocusBoardModel) dismiss() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	task := row.Task
	project := row.Project

	if msg := task.CanTransitionTo(models.StatusCanceled); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusCanceled)
	})
	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Dismissed #%d", task.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// toggleDone toggles between done and open.
func (m *FocusBoardModel) toggleDone() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}
	task := row.Task
	project := row.Project

	var targetStatus models.TaskStatus
	if task.Status.IsComplete() {
		targetStatus = models.StatusTodo
	} else {
		targetStatus = models.StatusDone
	}

	if msg := task.CanTransitionTo(targetStatus); msg != "" {
		m.statusMessage = msg
		return m, nil
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(targetStatus)
	})
	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
	} else {
		if targetStatus == models.StatusDone {
			m.statusMessage = fmt.Sprintf("Marked #%d done", task.ShortID)
		} else {
			m.statusMessage = fmt.Sprintf("Reopened #%d", task.ShortID)
		}
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// handleEnter opens URL or navigates to task.
func (m *FocusBoardModel) handleEnter() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil {
		return m, nil
	}

	// For all tasks, navigate to the task in its project
	if row.isTask() {
		return m, func() tea.Msg {
			return FocusBoardSelectMsg{Task: row.Task, Project: row.Project}
		}
	}

	// For actions: navigate to task or open URL
	if row.isAction() {
		if row.Task.URL != "" {
			return m, openBrowserCmd(row.Task.URL)
		}
		// No URL — open task detail
		return m, func() tea.Msg {
			return ShowTaskDetailMsg{Task: row.Task, Project: row.Project}
		}
	}

	return m, nil
}

// deleteItem deletes the action task at cursor.
func (m *FocusBoardModel) deleteItem() (tea.Model, tea.Cmd) {
	row := m.getRowAtCursor()
	if row == nil || !row.isAction() {
		return m, nil
	}

	task := row.Task
	project := row.Project

	// Clean up attachments
	_ = storage.CleanupTaskAttachments(project, task)

	// Remove task from project
	var remaining []*models.Task
	for _, t := range project.Tasks {
		if t.ID != task.ID {
			remaining = append(remaining, t)
		}
	}
	project.Tasks = remaining
	project.BuildTaskTree()

	if err := saveProjectSync(project); err != nil {
		m.statusMessage = "Failed to delete: " + err.Error()
	} else {
		m.statusMessage = fmt.Sprintf("Deleted #%d", task.ShortID)
		m.loadItems()
		m.clampCursor()
	}
	return m, nil
}

// openBrowserCmd opens a URL in the default browser.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}
