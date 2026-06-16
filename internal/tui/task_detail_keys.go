package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleKeyMsg dispatches key events for the task detail view.
func (m *TaskDetailModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle pending prefix keys
	if m.SkipModifierInPrefix(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		return m.handlePrefixKey(key)
	}

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
	case "D":
		return m, func() tea.Msg {
			return ShowTaskDecisionsMsg{Task: m.task, Project: m.project}
		}
	case "d":
		return m, func() tea.Msg {
			return SetDueDateMsg{Task: m.task, Project: m.project}
		}
	case "o":
		return m, func() tea.Msg {
			return CloseTaskDetailMsg{}
		}
	case "O":
		if m.task.URL != "" {
			return m, openBrowserCmd(m.task.URL)
		}
		return m, nil
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
		return m, helpCmd(ViewTaskDetail, "Task Detail", m.GetHelpItems())

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

	targetStatus, ok := StatusForPrefixKey(key)
	if !ok {
		return m, nil
	}

	pf := simplePersist(NewProjectService())
	result := applyStatusChange(pf, m.project, m.task.ID, targetStatus)
	if result.Err != nil {
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	if result.Cmd == nil {
		// No state change (validation message or no-op).
		if result.StatusMessage != "" {
			msg := result.StatusMessage
			return m, func() tea.Msg { return StatusMsg(msg) }
		}
		return m, nil
	}

	// Re-fetch task after mutation
	if fresh := m.project.GetTaskByID(m.task.ID); fresh != nil {
		m.task = fresh
	}

	if targetStatus == models.StatusDone {
		m.setCompletionFlash()
		return m, tea.Batch(result.Cmd, completionFlashScheduler())
	}
	m.setMutationFlash(statusMutationFlashBg(targetStatus))
	return m, tea.Batch(result.Cmd, mutationFlashTimeout())
}

// handlePriorityPrefix handles priority keys after the "!" prefix.
func (m *TaskDetailModel) handlePriorityPrefix(key string) (tea.Model, tea.Cmd) {
	if m.task == nil {
		return m, nil
	}

	priority, ok := PriorityForPrefixKey(key)
	if !ok {
		return m, nil
	}

	pf := simplePersist(NewProjectService())
	result := applyPriorityChange(pf, m.project, m.task.ID, priority)
	if result.Err != nil {
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	if result.StatusMessage != "" {
		msg := result.StatusMessage
		return m, func() tea.Msg { return StatusMsg(msg) }
	}

	// Re-fetch task after mutation
	if fresh := m.project.GetTaskByID(m.task.ID); fresh != nil {
		m.task = fresh
	}

	return m, nil
}

// getTaskRef returns the task and project for the current detail view.
func (m *TaskDetailModel) getTaskRef() (*models.Task, *models.Project) {
	return m.task, m.project
}

// HandledKeys returns every key the task detail view's Update can process.
// Matches the cases in handleKeyMsg plus prefix entry keys; status/priority
// suffix keys are reachable only via the "," and "!" prefixes and are not
// listed individually here.
func (m *TaskDetailModel) HandledKeys() []string {
	return []string{
		"esc", "enter",
		"j", "down", "k", "up",
		"ctrl+d", "ctrl+u",
		"G",
		"H",
		",", "!",
		"c", "l", "a", "h", "p", "S", "D", "d",
		"o", "O", "e", "g",
		"?",
	}
}

// GetHelpItems returns the help items for the task detail view.
func (m *TaskDetailModel) GetHelpItems() []HelpItem {
	items := GetKeybindingRegistry().GetAppLevelForView(ViewTaskDetail)
	return append(items, []HelpItem{
		{Key: "j/k", Description: "Scroll down/up", Category: "Navigation"},
		{Key: "Ctrl+D/U", Description: "Half page down/up", Category: "Navigation"},
		{Key: "g/G", Description: "Top/bottom (g also opens glow at top)", Category: "Navigation"},
		{Key: "o/Esc", Description: "Close detail view", Category: "Navigation"},
		{Key: "H", Description: "Toggle History cluster (collapse/expand)", Category: "Navigation"},
		{Key: ",s/d/b/x/o/r", Description: "Status: start/done/block/cancel/reopen/review", Category: "Actions"},
		{Key: "!1-5", Description: "Set priority", Category: "Actions"},
		{Key: "O", Description: "Open external URL", Category: "Actions"},
		{Key: "e", Description: "Edit task", Category: "Actions"},
		{Key: "d", Description: "Set due date", Category: "Actions"},
		{Key: "c", Description: "Comments", Category: "Sub-views"},
		{Key: "l", Description: "Checklist", Category: "Sub-views"},
		{Key: "a", Description: "Attachments", Category: "Sub-views"},
		{Key: "h", Description: "Activity history", Category: "Sub-views"},
		{Key: "p", Description: "Dependencies", Category: "Sub-views"},
		{Key: "S", Description: "Score editor", Category: "Sub-views"},
		{Key: "D", Description: "Decisions", Category: "Sub-views"},
	}...)
}
