package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleProjects()

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		// Handle confirmation mode
		if m.confirm.IsActive() {
			project := m.deleteTarget
			if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
				if confirmed && project != nil {
					m.deleteTarget = nil
					return m, m.deleteProject(project)
				}
				m.deleteTarget = nil
				return m, nil
			}
		}

		// Handle search mode
		if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
			visible = m.visibleProjects()
			m.cursor = ClampCursor(m.cursor, len(visible))
		}); handled {
			return model, cmd
		}

		// Dispatcher handles all registered keys (navigation, actions, prefixes).
		if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
			return model, cmd
		}

		switch msg.String() {
		case "esc":
			// Clear filters if active
			if m.filterContext != "" || m.filterCustomContext != "" || m.SearchQuery() != "" {
				m.filterContext = ""
				m.filterCustomContext = ""
				m.ClearSearch()
				return m, nil
			}
		case "?":
			return m, helpCmd(ViewDashboard, "Dashboard", m.GetHelpItems())
		}
		_ = visible
	}

	return m, nil
}

func (m *DashboardModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *DashboardModel) deleteProject(project *models.Project) tea.Cmd {
	dataDir := m.config.DataDir
	return func() tea.Msg {
		// Purge any existing trash entry with same slug
		if err := storage.PurgeFromTrash(project.Slug, dataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to purge old trash entry: %w", err)}
		}

		// Move project directory to trash
		if err := storage.MoveProjectToTrash(project, dataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to move project to trash: %w", err)}
		}

		return ProjectDeletedMsg{Project: project}
	}
}

func (m *DashboardModel) toggleProjectPinned(project *models.Project) tea.Cmd {
	project.TogglePinned()
	return func() tea.Msg {
		return ProjectPinnedToggledMsg{Project: project}
	}
}
