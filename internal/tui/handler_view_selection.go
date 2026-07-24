package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// handleViewSelectionMessages handles transient pickers, previews, and status editors.
func (m *AppModel) handleViewSelectionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowFilterMsg:
		if msg.CommonFilter != nil {
			m.showView(ViewFilter, NewFilterViewModelForOutliner(msg.CommonFilter, msg.Project))
		}
		return true, nil

	case ShowTaskJumperMsg:
		if msg.Project != nil {
			m.taskJumper = NewTaskJumperModel(msg.Project)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		if msg.CrossProject {
			m.taskJumper = NewCrossProjectTaskJumperModel(msg.Projects)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		return true, nil

	case TaskJumperSelectMsg:
		m.popView()
		if msg.Task != nil {
			switch m.currentView() { //nolint:exhaustive // Only outliner presentations accept jumper repositioning.
			case ViewCrossProjectOutliner:
				if m.crossOutliner != nil {
					m.crossOutliner.NavigateToTaskInProject(msg.Task, msg.Project)
				}
			case ViewOutliner:
				if m.outliner != nil {
					m.outliner.NavigateToTask(msg.Task)
				}
			}
		}
		return true, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectPreview, NewProjectPreviewModel(msg.Project, m.runtime))
		}
		return true, nil

	case ShowStatusInputMsg:
		if msg.Project != nil {
			m.showView(ViewStatusInput, NewStatusInputModel(msg.Project, m.runtime))
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
			if err := m.commitCatalogMutation(func(catalog *config.WorkspaceCatalog) {
				catalog.UpsertProject(msg.Project)
			}); err != nil {
				m.err = err
			}
		}

		m.showView(ViewStatusInput, NewStatusInputModel(msg.Project, m.runtime))
		return true, nil

	case StatusUpdatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.setSuccessStatus("Status updated")
		return true, tea.Batch(popCmd, m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())
	}

	return false, nil
}
