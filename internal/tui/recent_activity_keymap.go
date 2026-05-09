package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for RecentActivityModel.
func (m *RecentActivityModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			_, cmd := handleListNavigation("j", &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash)
			return m, cmd
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			_, cmd := handleListNavigation("k", &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash)
			return m, cmd
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			_, cmd := handleListNavigation("G", &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash)
			return m, cmd
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			_, cmd := handleListNavigation("ctrl+u", &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash)
			return m, cmd
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			_, cmd := handleListNavigation("ctrl+d", &m.cursor, len(m.filtered), m.Height(), &m.cursorFlash)
			return m, cmd
		},
	})...)

	entries = append(entries,
		keymap.Bind("tab", "Switch tab forward", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+left", &m.tabs, func() {
				m.applyFilter()
				m.clampCursor()
			})
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+right", &m.tabs, func() {
				m.applyFilter()
				m.clampCursor()
			})
			return m, nil
		}, keymap.AsHidden()),
	)

	// g prefix: gg = top
	entries = append(entries, keymap.Prefix("g", "Go…", "Navigation", []keymap.Entry{
		keymap.Bind("g", "Go to top", "Navigation", func() (tea.Model, tea.Cmd) {
			m.cursor = 0
			return m, nil
		}),
	}))

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Open task in project", "Actions", func() (tea.Model, tea.Cmd) {
			if m.cursor >= 0 && m.cursor < len(m.filtered) {
				dt := m.filtered[m.cursor]
				return m, func() tea.Msg { return RecentActivitySelectMsg(dt) }
			}
			return m, nil
		}),
		keymap.Bind("p", "Preview task", "Actions", func() (tea.Model, tea.Cmd) {
			if m.cursor >= 0 && m.cursor < len(m.filtered) {
				dt := m.filtered[m.cursor]
				return m, func() tea.Msg {
					return ShowRecentActivityPreviewMsg{Task: dt, Project: dt.Project}
				}
			}
			return m, nil
		}),
		keymap.Bind("r", "Refresh", "Actions", func() (tea.Model, tea.Cmd) {
			m.loadTasks()
			m.clampCursor()
			return m, nil
		}),
	)

	// Search
	entries = append(entries,
		keymap.Bind("/", "Filter tasks", "Filters", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
	)

	// Filter group: "F" prefix
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags: func() (tea.Model, tea.Cmd) {
			return m.handleRecentActivityFilterPrefix("#")
		},
		TogglePriority: func() (tea.Model, tea.Cmd) {
			return m.handleRecentActivityFilterPrefix("p")
		},
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) {
			return m.handleRecentActivityFilterPrefix("h")
		},
		ClearFilters: func() (tea.Model, tea.Cmd) {
			return m.handleRecentActivityFilterPrefix("x")
		},
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleRecentActivityFilterSubPrefix("Ft", key)
			}
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleRecentActivityFilterSubPrefix("Fs", key)
			}
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleRecentActivityFilterSubPrefix("Fd", key)
			}
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleRecentActivityFilterSubPrefix("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleRecentActivityFilterSubPrefix(prefix, key)
			}
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleRecentActivityFilterSubPrefix("FS", key)
			}
		},
	})...)

	// Standard task action groups: Status, Priority, DueDate, Snooze, Move,
	// LinkedTask, TaskPrefix, Stage, Blocker, CommonTaskAction.
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}
	hasItem := func() bool {
		t, _ := m.getTaskRef()
		return t != nil
	}
	entries = append(entries, buildStandardTaskActionEntries(TaskKeymapDeps{
		View:    m,
		HasItem: hasItem,
		Stages:  stages,
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			viewType := ViewRecentActivity
			viewName := "Recent Activity"
			if m.projectScope != nil {
				viewType = ViewProjectRecentActivity
				viewName = "Project Activity"
			}
			return m, helpCmd(viewType, viewName, m.GetHelpItems())
		},
		QuickNote: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
		},
	})...)

	return &keymap.KeyMap{
		ViewName: "RecentActivity",
		Entries:  entries,
	}
}
