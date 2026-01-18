package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleProjectMessages handles project-related messages (create, edit, archive, delete, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case SelectProjectMsg:
		// Pop command palette if that's where we came from
		if m.currentView() == ViewCommandPalette {
			m.popView()
		}
		m.switchToProject(msg.Project, true)
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil

	case CreateProjectMsg:
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.config, m.configManager))
		return true, nil

	case ProjectCreatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectArchivedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectPinnedToggledMsg:
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		statusMsg := "Pinned"
		if !msg.Project.Pinned {
			statusMsg = "Unpinned"
		}
		m.statusMessage = statusMsg
		return true, statusTimeout(2 * time.Second)

	case ProjectDeletedMsg:
		m.config.RemoveProject(msg.Project.ID)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectRestoredMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case EditProjectMsg:
		if msg.Project != nil {
			m.showView(ViewProjectEditor, NewProjectEditorModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ProjectUpdatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case SetCurrentProjectMsg:
		m.config.CurrentProject = msg.Project.ID
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = fmt.Sprintf("Set current: %s", msg.Project.Name)
		return true, statusTimeout(2 * time.Second)

	case ToggleRecentTasksMsg:
		newValue := !m.config.GetShowRecentTasks()
		m.config.SetShowRecentTasks(newValue)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if newValue {
			m.statusMessage = "Recent Tasks shown"
		} else {
			m.statusMessage = "Recent Tasks hidden"
		}
		return true, statusTimeout(2 * time.Second)
	}
	return false, nil
}
