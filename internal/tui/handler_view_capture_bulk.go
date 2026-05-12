package tui

import (
	"strings"

	"charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
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
			overlayWidth := max(min(m.width*3/5, m.width-4), 60)
			model.SetSize(overlayWidth, 0)
			m.quickCaptureOverlay = model
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
			firstProject := msg.Tasks[0].Project
			var tasks []*models.Task
			for _, item := range msg.Tasks {
				if item.Project.ID == firstProject.ID {
					tasks = append(tasks, item.Task)
				}
			}
			if len(tasks) > 0 {
				m.showView(ViewBulkTagSelector, NewBulkTagSelectorModel(tasks, firstProject))
			}
		}
		return true, nil

	case PaletteBulkDueDateMsg:
		if len(msg.Tasks) > 0 {
			m.pendingBulkTasks = msg.Tasks
			m.showView(ViewDateInput, NewDateInputModel(msg.Tasks[0].Task, msg.Tasks[0].Project))
		}
		return true, nil
	}

	return false, nil
}
