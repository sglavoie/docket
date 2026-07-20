package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleCaptureAndBulkMessages handles quick capture, bulk capture, bulk edit, and palette bulk messages.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowQuickCaptureMsg:
		// Contextual entry points (project tree, dashboard, outliner, kanban, and
		// their palette commands) supply the owning project. Context-free capture
		// falls back to the visible system Inbox and may still be rerouted with
		// %project inside the capture input.
		project := msg.Project
		if project == nil {
			project = m.projectByID(m.runtime.Catalog().SystemInboxProjectID)
		}
		if project != nil {
			var parts []string
			if msg.SuggestedTag != "" {
				parts = append(parts, "#"+msg.SuggestedTag)
			}
			if msg.SuggestedSection != "" {
				parts = append(parts, "/"+msg.SuggestedSection)
			}
			initialText := strings.Join(parts, " ")
			allProjects := m.activeProjects()
			if len(allProjects) == 0 {
				allProjects = msg.AllProjects
			}
			model := NewQuickCaptureModel(project, allProjects, false, initialText)
			m.showView(ViewQuickCapture, model)
		}
		return true, nil

	case ShowBulkCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewBulkCapture, NewBulkCaptureModel(msg.Project))
		}
		return true, nil

	case ShowBulkEditMsg:
		if len(msg.Tasks) > 0 && msg.Project != nil {
			m.showView(ViewBulkEdit, NewBulkEditModel(msg.Tasks, msg.Project))
		}
		return true, nil

	case BulkEditDoneMsg:
		m.popView()
		// project-view bulk/quick actions are handled by the outliner keymap
		return true, nil

	case PaletteBulkPriorityMsg:
		if len(msg.Tasks) > 0 {
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewPriorityPicker, NewPriorityPickerModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil

	case PaletteBulkTagMsg:
		if len(msg.Tasks) > 0 {
			refs := validTaskProjectRefs(msg.Tasks)
			if len(refs) > 0 {
				m.showView(ViewBulkTagSelector, NewBulkTagSelectorModel(refs))
			}
		}
		return true, nil

	case PaletteBulkDueDateMsg:
		if len(msg.Tasks) > 0 {
			return m.handleSetDueDate(SetDueDateMsg{TaskRefs: msg.Tasks})
		}
		return true, nil
	}

	return false, nil
}
