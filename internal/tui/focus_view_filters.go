package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// refreshAndClamp reloads data and clamps cursor after filter changes.
func (m *FocusViewModel) refreshAndClamp() { m.applyFilter(); m.clampCursor() }

func (m *FocusViewModel) buildTagCmd() tea.Cmd {
	tasks := make([]*models.Task, len(m.tasks))
	for i := range m.tasks {
		tasks[i] = m.tasks[i].Task
	}
	tags := buildTagListFromTasks(tasks)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: m.filter.Tags, Source: ViewFocusView, Title: "Filter by Tags"}
	}
}

// handleFocusFilterPrefix handles the second key after pressing F (filter prefix) in focus view.
func (m *FocusViewModel) handleFocusFilterPrefix(key string) (tea.Model, tea.Cmd) {
	if cmd, handled := handleFilterPrefixForView(m, key); handled {
		return m, cmd
	}
	return m, nil
}

// handleFocusFilterSubPrefix handles all filter sub-prefix keys (Ft, Fs, Fa, Fas-Fap, FS).
func (m *FocusViewModel) handleFocusFilterSubPrefix(prefix, key string) (tea.Model, tea.Cmd) {
	handleFilterSubPrefixForView(m, prefix, key)
	return m, nil
}
