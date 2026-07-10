package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handleCaptureAndBulkMessages handles quick capture, bulk capture, bulk edit, and palette bulk messages.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowQuickCaptureMsg:
		if msg.Project != nil {
			var parts []string
			if msg.SuggestedTag != "" {
				parts = append(parts, "#"+msg.SuggestedTag)
			}
			if msg.SuggestedSection != "" {
				parts = append(parts, "/"+msg.SuggestedSection)
			}
			initialText := strings.Join(parts, " ")
			model := NewQuickCaptureModel(msg.Project, msg.AllProjects, msg.FromGlobalCapture, initialText)
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
		if msg.Applied && m.projectTree != nil {
			return true, m.projectTree.applyBulkChanges(msg.Changes, msg.Tasks)
		}
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
