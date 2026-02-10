package tui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleKeyMsg dispatches key events for the task detail view.
func (m *TaskDetailModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

	switch key {
	// Close view
	case "q", "esc":
		return m, func() tea.Msg {
			return CloseTaskDetailMsg{}
		}

	// Scrolling
	case "j", "down":
		m.scrollOffset++
		m.clampScroll()
		return m, nil
	case "k", "up":
		m.scrollOffset--
		m.clampScroll()
		return m, nil
	case "ctrl+d":
		m.scrollOffset += m.viewHeight() / 2
		m.clampScroll()
		return m, nil
	case "ctrl+u":
		m.scrollOffset -= m.viewHeight() / 2
		m.clampScroll()
		return m, nil
	case "G":
		m.scrollOffset = m.totalLines
		m.clampScroll()
		return m, nil

	// Prefix keys
	case ",":
		m.pendingPrefix = ","
		return m, nil
	case "!":
		m.pendingPrefix = "!"
		return m, nil

	// Sub-view launchers
	case "c":
		return m, func() tea.Msg {
			return ShowTaskCommentsMsg{Task: m.task, Project: m.project}
		}
	case "l":
		return m, func() tea.Msg {
			return ShowTaskChecklistMsg{Task: m.task, Project: m.project}
		}
	case "a":
		return m, func() tea.Msg {
			return ShowTaskAttachmentsMsg{Task: m.task, Project: m.project}
		}
	case "h":
		return m, func() tea.Msg {
			return OpenTaskActivityMsg{Task: m.task, Project: m.project}
		}
	case "p":
		return m, func() tea.Msg {
			return ShowDependencyViewerMsg{Task: m.task, Project: m.project}
		}
	case "S":
		return m, func() tea.Msg {
			return ShowScoreEditorMsg{Task: m.task, Project: m.project}
		}
	case "d":
		return m, func() tea.Msg {
			return SetDueDateMsg{Task: m.task, Project: m.project}
		}
	case "e":
		return m, func() tea.Msg {
			return EditTaskMsg{Task: m.task}
		}
	case "g":
		if m.scrollOffset > 0 {
			// g goes to top when not already at top
			m.scrollOffset = 0
			return m, nil
		}
		// At top, open glow
		return m, func() tea.Msg {
			return GlowViewMsg{Task: m.task}
		}

	// Help
	case "?":
		return m, func() tea.Msg {
			return ShowHelpMsg{
				SourceView: ViewTaskDetail,
				ViewName:   "Task Detail",
				Items:      m.GetHelpItems(),
			}
		}

	// Clipboard
	case "y":
		return m, func() tea.Msg {
			return CopyTaskTitleMsg{Task: m.task}
		}
	case "Y":
		return m, func() tea.Msg {
			return CopyTaskDetailsMsg{Task: m.task}
		}
	case "ctrl+y":
		return m, func() tea.Msg {
			return CopyTaskTitleAndDescMsg{Task: m.task}
		}
	}

	return m, nil
}

// handlePrefixKey handles keys after a prefix character.
func (m *TaskDetailModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = ""

	switch prefix {
	case ",":
		return m.handleStatusPrefix(key)
	case "!":
		return m.handlePriorityPrefix(key)
	}

	return m, nil
}

// handleStatusPrefix handles status change keys after the "," prefix.
func (m *TaskDetailModel) handleStatusPrefix(key string) (tea.Model, tea.Cmd) {
	if m.task == nil {
		return m, nil
	}

	task := m.task
	project := m.project

	var action string
	switch key {
	case "s":
		action = "start"
	case "d":
		action = "done"
	case "b":
		action = "block"
	case "x":
		action = "cancel"
	case "o":
		action = "reopen"
	case "r":
		action = "review"
	default:
		return m, nil
	}

	// Apply status change via MutateTask for pointer safety
	project.MutateTask(task.ID, func(t *models.Task) {
		switch action {
		case "start":
			t.Start()
		case "done":
			t.Complete()
		case "block":
			t.Block()
		case "cancel":
			t.Cancel()
		case "reopen":
			t.Reopen()
		case "review":
			t.Review()
		}
	})

	// Re-fetch task after mutation
	if fresh := project.GetTaskByID(task.ID); fresh != nil {
		m.task = fresh
	}

	return m, tea.Batch(
		startLoading("Saving..."),
		SaveProjectAsync(project, fmt.Sprintf("Task %s: %s", action, task.Title)),
	)
}

// handlePriorityPrefix handles priority keys after the "!" prefix.
func (m *TaskDetailModel) handlePriorityPrefix(key string) (tea.Model, tea.Cmd) {
	if m.task == nil {
		return m, nil
	}

	priority, err := strconv.Atoi(key)
	if err != nil || priority < 1 || priority > 5 {
		return m, nil
	}

	task := m.task
	project := m.project

	project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	if fresh := project.GetTaskByID(task.ID); fresh != nil {
		m.task = fresh
	}

	return m, tea.Batch(
		startLoading("Saving..."),
		SaveProjectAsync(project, fmt.Sprintf("Priority set to %d", priority)),
	)
}

// GetHelpItems returns the help items for the task detail view.
func (m *TaskDetailModel) GetHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "j/k", Description: "Scroll down/up", Category: "Navigation"},
		{Key: "Ctrl+d/u", Description: "Half page down/up", Category: "Navigation"},
		{Key: "g/G", Description: "Top/bottom (g also opens glow at top)", Category: "Navigation"},
		{Key: "q/Esc", Description: "Close detail view", Category: "Navigation"},
		{Key: ",s/d/b/x/o/r", Description: "Status: start/done/block/cancel/reopen/review", Category: "Actions"},
		{Key: "!1-5", Description: "Set priority", Category: "Actions"},
		{Key: "e", Description: "Edit task", Category: "Actions"},
		{Key: "d", Description: "Set due date", Category: "Actions"},
		{Key: "c", Description: "Comments", Category: "Sub-views"},
		{Key: "l", Description: "Checklist", Category: "Sub-views"},
		{Key: "a", Description: "Attachments", Category: "Sub-views"},
		{Key: "h", Description: "Activity history", Category: "Sub-views"},
		{Key: "p", Description: "Dependencies", Category: "Sub-views"},
		{Key: "S", Description: "Score editor", Category: "Sub-views"},
		{Key: "y", Description: "Copy title", Category: "Clipboard"},
		{Key: "Y", Description: "Copy description", Category: "Clipboard"},
		{Key: "Ctrl+y", Description: "Copy title and description", Category: "Clipboard"},
	}
}
