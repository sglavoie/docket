package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// handleKeyMsg dispatches key events for the task detail view.
func (m *TaskDetailModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.task == nil {
		return m, nil
	}

	switch key {
	// Close view
	case "esc", "enter":
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
	case "H":
		m.historyExpanded = !m.historyExpanded
		// Re-clamp scroll: total lines change when the History cluster
		// expands or collapses, so the previous offset may now exceed bounds.
		m.clampScroll()
		return m, nil

	case "d":
		return m, func() tea.Msg {
			return SetDueDateMsg{Task: m.task, Project: m.project}
		}
	case "o":
		return m, func() tea.Msg {
			return CloseTaskDetailMsg{}
		}
	case "O":
		return m, nil
	case "e":
		return m, func() tea.Msg {
			return EditTaskMsg{Task: m.task, Project: m.project}
		}
	case "E":
		return m, func() tea.Msg {
			return ShowTaskTextEditorMsg{
				Task: m.task, Project: m.project, Operation: app.TaskOperationEditDescription,
			}
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
		return m, helpCmd(ViewTaskDetail, "Task Detail", m.GetHelpItems())

	}

	return m, nil
}

// getTaskRef returns the task and project for the current detail view.
func (m *TaskDetailModel) getTaskRef() (*models.Task, *models.Project) {
	return m.task, m.project
}

// HandledKeys returns every key the task detail view's Update can process.
// Matches the direct cases in handleKeyMsg.
func (m *TaskDetailModel) HandledKeys() []string {
	return []string{
		"esc", "enter",
		"j", "down", "k", "up",
		"ctrl+d", "ctrl+u",
		"G",
		"H",
		"d",
		"o", "O", "e", "E", "g",
		"?",
	}
}

// GetHelpItems returns the help items for the task detail view.
func (m *TaskDetailModel) GetHelpItems() []HelpItem {
	if m.tab != TaskOverview {
		var items []HelpItem
		switch m.tab {
		case TaskOverview:
		case TaskChecklist:
			items = helpItemsFromKeyMap(m.checklist.buildKeyMap())
		case TaskComments:
			items = helpItemsFromKeyMap(m.comments.buildKeyMap())
		case TaskDecisions:
			items = helpItemsFromKeyMap(m.decisions.buildKeyMap())
		case TaskAttachments:
			items = helpItemsFromKeyMap(m.attachments.buildKeyMap())
		default:
		}
		return items
	}
	items := getGlobalHelpItems()
	return append(items, []HelpItem{
		{Key: "j/k", Description: "Scroll down/up", Category: "Navigation"},
		{Key: "Ctrl+D/U", Description: "Half page down/up", Category: "Navigation"},
		{Key: "g/G", Description: "Top/bottom (g also opens glow at top)", Category: "Navigation"},
		{Key: "o/Esc", Description: "Close detail view", Category: "Navigation"},
		{Key: "H", Description: "Toggle History cluster (collapse/expand)", Category: "Navigation"},
		{Key: ",s/d/b/x/o/r", Description: "State: start/done/waiting/cancel/reopen/review", Category: "Actions"},
		{Key: "!0-3", Description: "Set priority", Category: "Actions"},
		{Key: "O", Description: "Open external URL", Category: "Actions"},
		{Key: "e/E", Description: "Edit title/description", Category: "Actions"},
		{Key: "d", Description: "Set due date", Category: "Actions"},
	}...)
}
