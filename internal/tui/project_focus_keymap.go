package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for ProjectFocusViewModel.
func (m *ProjectFocusViewModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.cfg != nil {
		stages = m.cfg.Stages
	}

	hasItem := func() bool { return m.getTaskAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			tasks := m.filteredTasks()
			if m.cursor < len(tasks)-1 {
				m.cursor++
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			tasks := m.filteredTasks()
			if len(tasks) > 0 {
				m.cursor = len(tasks) - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filteredTasks()), -HalfPage(m.ContentHeight()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filteredTasks()), HalfPage(m.ContentHeight()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)

	// --- Goto group: g prefix (gg = top only, no gp) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			// No gp action for this view
			return m, nil
		},
	})...)

	// --- Standard task action groups ---
	entries = append(entries, buildStandardTaskActionEntries(TaskKeymapDeps{
		View:    m,
		HasItem: hasItem,
		Stages:  stages,
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(ViewProjectFocusView, "Project Focus", m.GetHelpItems())
		},
		ToggleActive: func() (tea.Model, tea.Cmd) { return m.toggleActive() },
		DeleteTask:   func() (tea.Model, tea.Cmd) { return m, nil },
	})...)

	// --- Filter group: "F" prefix ---
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("#") },
		TogglePriority:    func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("p") },
		ToggleHideDimmed:  func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("h") },
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("i") },
		ClearFilters:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Ft", key) }
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fd", key) }
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fa", key) }
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewProjectFocusView} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
	})...)

	// --- View-specific direct bindings ---

	// Sub-tab row focus
	entries = append(entries,
		keymap.Bind("shift+down", "Enter sub-tab row focus", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				m.subTabRowFocused = true
			}
			return m, nil
		}),
		keymap.Bind("shift+up", "Exit sub-tab row focus", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				m.subTabRowFocused = false
			}
			return m, nil
		}),
	)

	// Tab / sub-tab navigation
	entries = append(entries,
		keymap.Bind("tab", "Next sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
				m.resetViewScrollOffset()
				m.clampCursor()
			}
			return m, nil
		}),
		keymap.Bind("shift+right", "Next sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
				m.resetViewScrollOffset()
				m.clampCursor()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
				m.resetViewScrollOffset()
				m.clampCursor()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+left", "Previous sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
				m.resetViewScrollOffset()
				m.clampCursor()
			}
			return m, nil
		}, keymap.AsHidden()),
	)

	// Enter: close and return to project tree
	entries = append(entries,
		keymap.Bind("enter", "Close and return to project tree", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CloseProjectFocusViewMsg{} }
		}),
	)

	// Refresh
	entries = append(entries,
		keymap.Bind("r", "Refresh", "View", func() (tea.Model, tea.Cmd) {
			_ = defaultPersister.Load(m.project)
			m.loadTasks()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// Priority direct keys: 1-5
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return m.changeTaskPriority(key)
			}, keymap.AsHidden()),
		)
	}

	return &keymap.KeyMap{
		ViewName: "Project Focus",
		Entries:  entries,
	}
}
