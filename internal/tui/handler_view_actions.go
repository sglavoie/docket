package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleViewActionMessages handles task action views such as priority, capture, blockers, and attachments.
func (m *AppModel) handleViewActionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowPriorityViewMsg:
		if msg.Project != nil {
			m.showView(ViewPriorityView, NewPriorityViewModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ClosePriorityViewMsg:
		m.popView()
		return true, nil

	case PriorityViewSelectMsg:
		m.popView()
		if msg.Task != nil && m.currentProject != nil {
			return true, m.navigateToTaskInProject(msg.Task, m.currentProject)
		}
		return true, nil

	case ShowQuickCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewQuickCapture, NewQuickCaptureModel(msg.Project, msg.AllProjects))
		}
		return true, nil

	case CloseQuickCaptureMsg:
		m.popView()
		return true, nil

	case ShowBulkCaptureMsg:
		if msg.Project != nil {
			m.showView(ViewBulkCapture, NewBulkCaptureModel(msg.Project))
		}
		return true, nil

	case CloseBulkCaptureMsg:
		m.popView()
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

// handleViewSupportMessages handles blockers and attachment views.
func (m *AppModel) handleViewSupportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowBlockersMsg:
		if msg.Project != nil {
			editor := m.config.Editor
			m.showView(ViewBlockers, NewBlockersModelWithEditor(msg.Project, editor))
		}
		return true, nil

	case CloseBlockersMsg:
		m.popView()
		return true, nil

	case ShowGlobalBlockersMsg:
		m.showView(ViewGlobalBlockers, NewGlobalBlockersModel(m.config))
		return true, nil

	case CloseGlobalBlockersMsg:
		m.popToAnchorView()
		return true, nil

	case BlockerGoToProjectMsg:
		m.popView()
		if msg.Project != nil {
			m.switchToProject(msg.Project, true)
			m.pushView(ViewProjectTree)
		}
		return true, nil

	case BlockerAddedMsg, BlockerEditedMsg, BlockerDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowTaskAttachmentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskAttachments, NewTaskAttachmentsModel(msg.Task, msg.Project, m.config))
		}
		return true, nil

	case CloseTaskAttachmentsMsg:
		m.popView()
		return true, nil

	case AttachmentAddedMsg, AttachmentDeletedMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case AttachmentReorderedMsg:
		return true, nil
	}

	return false, nil
}
