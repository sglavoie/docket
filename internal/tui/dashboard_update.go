package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
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

		if m.SkipModifierInPrefix(msg) {
			return m, nil
		}
		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

		switch msg.String() {
		case "/":
			return m, m.ActivateSearch()
		case "esc":
			// Clear filters if active
			if m.filterContext != "" || m.filterCustomContext != "" || m.SearchQuery() != "" {
				m.filterContext = ""
				m.filterCustomContext = ""
				m.ClearSearch()
				return m, nil
			}
		case "up", "k", "down", "j", "G", "pgup", "pgdown":
			_, navCmd := handleListNavigation(msg.String(), &m.cursor, len(visible), m.height, &m.cursorFlash)
			if navCmd != nil {
				return m, navCmd
			}
		case "enter":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: project}
				}
			}
		case "?":
			return m, helpCmd(ViewDashboard, "Dashboard", m.GetHelpItems())
		case "R":
			m.loadProjects()
		case "n":
			return m, func() tea.Msg {
				return CreateProjectMsg{}
			}
		case "a":
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
		case "S":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowStatusIconPickerMsg{Project: project}
				}
			}
		case "D":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				if project.Sandbox {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("cannot delete the sandbox project")}
					}
				}
				m.deleteTarget = project
				m.confirm.Show("delete", fmt.Sprintf("Delete project '%s'?", project.Name))
			}
		case "e":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return EditProjectMsg{Project: project}
				}
			}
		case "p":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowProjectPreviewMsg{Project: project}
				}
			}
		case "C":
			// Open context picker popup
			return m, func() tea.Msg {
				return ShowContextPickerMsg{Dashboard: m}
			}
		case "ctrl+t":
			return m, func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			}
		case "@":
			return m, func() tea.Msg {
				return ShowProjectCycleManagerMsg{}
			}
		case "`":
			return m, func() tea.Msg {
				return CycleProjectMsg{Forward: true}
			}
		case "~":
			return m, func() tea.Msg {
				return CycleProjectMsg{Forward: false}
			}
		case "ctrl+p":
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, m.toggleProjectPinned(project)
			}
		case "T":
			return m, func() tea.Msg {
				return ShowTrashedProjectsMsg{}
			}
		case "b":
			// Open blockers for selected project
			if len(visible) > 0 && m.cursor < len(visible) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowBlockersMsg{Project: project}
				}
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Quick select pinned project by number
			n := int(msg.String()[0] - '0')
			pinnedCount := 0
			for _, p := range visible {
				if p.Pinned {
					pinnedCount++
					if pinnedCount == n {
						project := p // capture for closure
						return m, func() tea.Msg {
							return SelectProjectMsg{Project: project}
						}
					}
				}
			}
		case "g":
			m.pendingPrefix = "g"
		case "ctrl+e":
			project := m.SelectedProject()
			return m, func() tea.Msg { return ExportProjectMsg{Project: project} }
		}
	}

	return m, nil
}

func (m *DashboardModel) HandledKeys() []string {
	return []string{
		// Navigation
		"j", "k", "up", "down", "G", "g", "enter", "/",
		// Actions
		"R", "n", "a", "S", "D", "e", "p",
		"C", "ctrl+t", "@", "`", "~", "ctrl+p", "T", "b", "ctrl+e",
		// Quick-select pinned projects
		"1", "2", "3", "4", "5", "6", "7", "8", "9",
	}
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
