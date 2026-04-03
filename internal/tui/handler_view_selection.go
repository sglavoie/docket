package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
)

// handleViewSelectionMessages handles transient pickers, previews, and status editors.
func (m *AppModel) handleViewSelectionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowFilterMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewFilter, NewFilterViewModel(msg.ProjectTree))
		} else if msg.CommonFilter != nil {
			m.showView(ViewFilter, NewFilterViewModelForOutliner(msg.CommonFilter, msg.Project))
		}
		return true, nil

	case CloseFilterMsg:
		m.popView()
		return true, nil

	case ShowTaskJumperMsg:
		if msg.Project != nil {
			m.taskJumper = NewTaskJumperModel(msg.Project)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		return true, nil

	case TaskJumperSelectMsg:
		m.popView()
		if msg.Task != nil {
			switch m.currentView() {
			case ViewOutliner:
				if m.outliner != nil {
					m.outliner.NavigateToSidebarTask(msg.Task)
				}
			default:
				if m.projectTree != nil {
					return true, m.projectTree.NavigateToTask(msg.Task)
				}
			}
		}
		return true, nil

	case CloseTaskJumperMsg:
		m.popView()
		return true, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectPreview, NewProjectPreviewModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case CloseProjectPreviewMsg:
		m.popView()
		return true, nil

	case ShowStatusInputMsg:
		if msg.Project != nil {
			m.showView(ViewStatusInput, NewStatusInputModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ShowStatusIconPickerMsg:
		if msg.Project != nil {
			m.showView(ViewStatusIconPicker, NewStatusIconPickerModel(msg.Project))
		}
		return true, nil

	case StatusIconSelectedMsg:
		m.popView()
		if msg.Project == nil {
			return true, nil
		}

		if !msg.Canceled {
			msg.Project.StatusIcon = msg.Key
			if msg.Key != "" {
				msg.Project.StatusIconSource = "user"
			} else {
				msg.Project.StatusIconSource = ""
			}
			msg.Project.UpdatedAt = time.Now()
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
		}

		m.showView(ViewStatusInput, NewStatusInputModel(msg.Project, m.config, m.configManager))
		return true, nil

	case StatusUpdatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.loadProjects()
		}
		m.setSuccessStatus("Status updated")
		return true, defaultStatusTimeout()
	}

	return false, nil
}
