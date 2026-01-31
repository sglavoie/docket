package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	visible := m.visibleProjects()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle confirmation mode
		if m.confirm.IsActive() {
			project := m.deleteTarget
			if confirmed, handled := m.confirm.HandleKey(msg.String()); handled {
				if confirmed && project != nil {
					m.deleteTarget = nil
					return m, m.deleteProject(project)
				}
				m.deleteTarget = nil
				return m, nil
			}
		}

		// Handle search mode
		if m.IsSearchMode() {
			handled, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				visible = m.visibleProjects()
				m.cursor = ClampCursor(m.cursor, len(visible))
			}
			if handled {
				return m, cmd
			}
			return m, cmd
		}

		switch msg.String() {
		case "\\":
			return m, m.ActivateSearch()
		case "/":
			return m, func() tea.Msg {
				return ShowGlobalSearchMsg{}
			}
		case "esc":
			// Clear filters if active
			if m.filterContext != "" || m.filterCustomContext != "" || m.SearchQuery() != "" {
				m.filterContext = ""
				m.filterCustomContext = ""
				m.ClearSearch()
				return m, nil
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(visible)-1 {
				m.cursor++
			}
		case "enter":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: visible[m.cursor]}
				}
			}
		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewDashboard,
					ViewName:   "Dashboard",
					Items:      m.GetHelpItems(),
				}
			}
		case "ctrl+r":
			m.loadProjects()
		case "R":
			return m, func() tea.Msg {
				return ToggleRecentTasksMsg{}
			}
		case "n":
			return m, func() tea.Msg {
				return CreateProjectMsg{}
			}
		case "N":
			return m, func() tea.Msg {
				return ShowGlobalNotesMsg{}
			}
		case "a":
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
		case ",":
			return m, func() tea.Msg {
				return SettingsMsg{}
			}
		case "S":
			sandbox := m.config.GetSandboxProject()
			if sandbox != nil {
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: sandbox}
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
				m.confirm.Show("delete", fmt.Sprintf("Delete project '%s'? (y/n)", project.Name))
			}
		case "e":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return EditProjectMsg{Project: visible[m.cursor]}
				}
			}
		case "p":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return ShowProjectPreviewMsg{Project: visible[m.cursor]}
				}
			}
		case "c":
			// Open context picker popup
			return m, func() tea.Msg {
				return ShowContextPickerMsg{Dashboard: m}
			}
		case "ctrl+d":
			return m, func() tea.Msg {
				return ShowDueDateOverviewMsg{}
			}
		case "r":
			return m, func() tea.Msg {
				return ShowDailyReviewMsg{}
			}
		case "f":
			return m, func() tea.Msg {
				return ShowFocusViewMsg{}
			}
		case "l":
			return m, func() tea.Msg {
				return ShowRecentActivityMsg{}
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
		case "s":
			if CursorInBounds(m.cursor, len(visible)) {
				return m, func() tea.Msg {
					return SetCurrentProjectMsg{Project: visible[m.cursor]}
				}
			}
		case "T":
			return m, func() tea.Msg {
				return ShowTrashedProjectsMsg{}
			}
		case "W":
			return m, func() tea.Msg {
				return ShowWeeklyReviewMsg{}
			}
		case "w":
			return m, func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			}
		case "b":
			// Open blockers for selected project
			if len(visible) > 0 && m.cursor < len(visible) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowBlockersMsg{Project: project}
				}
			}
		case "|":
			if !m.config.GetTutorDismissed() {
				return m, func() tea.Msg {
					return TutorDismissedMsg{}
				}
			}
		case "B":
			return m, func() tea.Msg {
				return ShowGlobalBlockersMsg{}
			}
		case "ctrl+w":
			return m, func() tea.Msg {
				return ShowWorkflowRulesMsg{}
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
			// Go to first item
			m.cursor = 0
			return m, nil
		case "G":
			// Go to last item (Vim-style)
			if len(visible) > 0 {
				m.cursor = len(visible) - 1
			}
		}
	}

	return m, nil
}

func (m *DashboardModel) deleteProject(project *models.Project) tea.Cmd {
	return func() tea.Msg {
		// Purge any existing trash entry with same slug
		if err := storage.PurgeFromTrash(project.Slug, m.config.DataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to purge old trash entry: %w", err)}
		}

		// Move project directory to trash
		if err := storage.MoveProjectToTrash(project, m.config.DataDir); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to move project to trash: %w", err)}
		}

		return ProjectDeletedMsg{Project: project}
	}
}

func (m *DashboardModel) toggleProjectPinned(project *models.Project) tea.Cmd {
	return func() tea.Msg {
		project.TogglePinned()
		return ProjectPinnedToggledMsg{Project: project}
	}
}
