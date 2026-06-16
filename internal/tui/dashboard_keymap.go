package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for DashboardModel.
func (m *DashboardModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			_, cmd := handleListNavigation("j", &m.cursor, len(visible), m.height, &m.cursorFlash)
			return m, cmd
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			_, cmd := handleListNavigation("k", &m.cursor, len(visible), m.height, &m.cursorFlash)
			return m, cmd
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			_, cmd := handleListNavigation("down", &m.cursor, len(visible), m.height, &m.cursorFlash)
			return m, cmd
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			_, cmd := handleListNavigation("up", &m.cursor, len(visible), m.height, &m.cursorFlash)
			return m, cmd
		}, keymap.AsHidden()),
		keymap.Bind("G", "Go to last", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			_, cmd := handleListNavigation("G", &m.cursor, len(visible), m.height, &m.cursorFlash)
			return m, cmd
		}),
		keymap.Bind("/", "Search projects", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
		keymap.Bind("ctrl+t", "Global tag manager", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowGlobalTagManagerMsg{} }
		}),
		keymap.Bind("b", "Project blockers", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if len(visible) > 0 && m.cursor < len(visible) {
				project := visible[m.cursor]
				return m, func() tea.Msg { return ShowBlockersMsg{Project: project} }
			}
			return m, nil
		}),
	)

	// g prefix: gg = top, gk = kanban
	entries = append(entries, keymap.Prefix("g", "Go…", "Navigation", []keymap.Entry{
		keymap.Bind("g", "Go to top", "Navigation", func() (tea.Model, tea.Cmd) {
			_, cmd := handleGotoTopPrefix("g", &m.cursor, &m.cursorFlash)
			return m, cmd
		}),
		keymap.Bind("k", "Open kanban for selected project", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg { return ShowKanbanMsg{Project: project} }
			}
			return m, func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
		}),
	}))

	// ; prefix: ;W = workflow rules
	entries = append(entries, keymap.Prefix(";", "Workflow…", "Navigation", []keymap.Entry{
		keymap.Bind("W", "Workflow rules", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }),
	}))

	// , prefix: ,c = toggle compact info cards
	entries = append(entries, keymap.Prefix(",", "View…", "View", []keymap.Entry{
		keymap.Bind("c", "Toggle compact info cards", "View", func() (tea.Model, tea.Cmd) {
			m.dashboardCompactCards = !m.dashboardCompactCards
			return m, nil
		}),
	}))

	// @ opens the cycle manager popup (sub-keys are handled inside the popup itself).
	entries = append(entries,
		keymap.Bind("@", "Open cycle manager", "Project Cycling", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectCycleManagerMsg{} }
		}),
	)

	// Project cycling
	entries = append(entries,
		keymap.Bind("`", "Open project picker", "Project Cycling", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectCyclePickerMsg{} }
		}),
		keymap.Bind("~", "Previous project in cycle", "Project Cycling", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CycleProjectMsg{Forward: false} }
		}),
	)

	// Filters
	entries = append(entries,
		keymap.Bind("C", "Filter by context", "Filters", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowContextPickerMsg{Dashboard: m} }
		}),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Select project", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg { return SelectProjectMsg{Project: project} }
			}
			return m, nil
		}),
		keymap.Bind("n", "New project", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CreateProjectMsg{} }
		}),
		keymap.Bind("e", "Edit project", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg { return EditProjectMsg{Project: project} }
			}
			return m, nil
		}),
		keymap.Bind("p", "Preview project", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg { return ShowProjectPreviewMsg{Project: project} }
			}
			return m, nil
		}),
		keymap.Bind("D", "Delete project", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
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
			return m, nil
		}),
		keymap.Bind("ctrl+p", "Toggle pinned", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, m.toggleProjectPinned(project)
			}
			return m, nil
		}),
		keymap.Bind("a", "Toggle archived filter", "Actions", func() (tea.Model, tea.Cmd) {
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
			return m, nil
		}),
		keymap.Bind("u", "Unarchive project", "Actions", func() (tea.Model, tea.Cmd) {
			project := m.SelectedProject()
			if project == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return ProjectArchiveRequestMsg{Project: project, Archive: false}
			}
		}, keymap.WithGate(func() bool { return m.showArchived })),
		keymap.Bind("T", "View trash", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowTrashedProjectsMsg{} }
		}),
		keymap.Bind("S", "Set status icon & note", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg { return ShowStatusIconPickerMsg{Project: project} }
			}
			return m, nil
		}),
		keymap.Bind("R", "Refresh", "Actions", func() (tea.Model, tea.Cmd) {
			m.loadProjects()
			return m, nil
		}),
		keymap.Bind("ctrl+e", "Export project to Markdown", "Actions", func() (tea.Model, tea.Cmd) {
			project := m.SelectedProject()
			return m, func() tea.Msg { return ExportProjectMsg{Project: project} }
		}),
		keymap.Bind("i", "Quick capture", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowQuickCaptureMsg{Project: project, AllProjects: m.config.GetActiveProjects()}
				}
			}
			return m, nil
		}),
		keymap.Bind("I", "Bulk capture", "Actions", func() (tea.Model, tea.Cmd) {
			visible := m.visibleProjects()
			if CursorInBounds(m.cursor, len(visible)) {
				project := visible[m.cursor]
				return m, func() tea.Msg {
					return ShowBulkCaptureMsg{Project: project}
				}
			}
			return m, nil
		}),
		// Quick-select pinned projects
		keymap.Bind("1", "Quick select pinned 1", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(1)
		}, keymap.AsHidden()),
		keymap.Bind("2", "Quick select pinned 2", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(2)
		}, keymap.AsHidden()),
		keymap.Bind("3", "Quick select pinned 3", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(3)
		}, keymap.AsHidden()),
		keymap.Bind("4", "Quick select pinned 4", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(4)
		}, keymap.AsHidden()),
		keymap.Bind("5", "Quick select pinned 5", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(5)
		}, keymap.AsHidden()),
		keymap.Bind("6", "Quick select pinned 6", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(6)
		}, keymap.AsHidden()),
		keymap.Bind("7", "Quick select pinned 7", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(7)
		}, keymap.AsHidden()),
		keymap.Bind("8", "Quick select pinned 8", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(8)
		}, keymap.AsHidden()),
		keymap.Bind("9", "Quick select pinned 9", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.quickSelectPinned(9)
		}, keymap.AsHidden()),
	)

	return &keymap.KeyMap{
		ViewName: "Dashboard",
		Entries:  entries,
	}
}

// quickSelectPinned selects the nth pinned project (1-based).
func (m *DashboardModel) quickSelectPinned(n int) tea.Cmd {
	visible := m.visibleProjects()
	pinnedCount := 0
	for _, p := range visible {
		if p.Pinned {
			pinnedCount++
			if pinnedCount == n {
				project := p
				return func() tea.Msg { return SelectProjectMsg{Project: project} }
			}
		}
	}
	return nil
}
