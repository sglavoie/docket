package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildFocusBoardKeyMap constructs the declarative KeyMap for FocusBoardModel.
func buildFocusBoardKeyMap(m *FocusBoardModel) *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.skipGroupHeaderForward()
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
				m.skipGroupHeaderBackward()
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filtered), -HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filtered), HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: m.focusBoardGoToProject,
	})...)
	entries = append(entries,
		keymap.Bind("n", "Go to next parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.focusBoardGoToNextParent()
		}),
		keymap.Bind("N", "Go to previous parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.focusBoardGoToPrevParent()
		}),
	)

	// --- View-specific direct bindings ---

	// Tab navigation
	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
	)

	// Search
	entries = append(entries,
		keymap.Bind("/", "Search", "Filtering", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("/"))
		}),
	)

	// Quick-add
	entries = append(entries,
		keymap.Bind("a", "Quick-add board item", "Actions", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("a"))
		}),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "View task details", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("enter"))
		}),
		keymap.Bind("x", "Complete or dismiss item", "Actions", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("x"))
		}),
		keymap.Bind("p", "Plan or set priority", "Actions", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("p"))
		}),
		keymap.Bind("P", "Park task", "Actions", func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowParkInputMsg{Task: task, Project: project} }
		}),
	)

	entries = append(entries, buildStandardTaskActionEntries(m.focusBoardTaskKeymapDeps())...)

	// --- Today sort keys: 1-5, 0 ---
	entries = append(entries, focusBoardSortGroup(keymap.SortDeps{
		SortByPriority: m.focusBoardSortAction("1"),
		SortByDueDate:  m.focusBoardSortAction("2"),
		SortByCreated:  m.focusBoardSortAction("3"),
		SortByStatus:   m.focusBoardSortAction("4"),
		SortByScore:    m.focusBoardSortAction("5"),
		ResetSort:      m.focusBoardSortAction("0"),
	})...)

	entries = append(entries, keymap.Prefix("f", "Preview", "View", []keymap.Entry{
		keymap.Bind("p", "Toggle inline preview", "View", m.focusBoardTogglePreview),
	}))

	// --- Shared filter group: "F" prefix ---
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("#") },
		TogglePriority:    func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("p") },
		ToggleHideDimmed:  func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("h") },
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("i") },
		ToggleParked:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("k") },
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
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewFocusBoard} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("F.", key) }
		},
		StageFilterNames: stages,
	})...)

	return &keymap.KeyMap{
		ViewName: "Today",
		Entries:  entries,
	}
}

func (m *FocusBoardModel) focusBoardTaskKeymapDeps() TaskKeymapDeps {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}
	return TaskKeymapDeps{
		View:                m,
		HasItem:             m.focusBoardHasItem,
		Stages:              stages,
		ToggleHelp:          m.toggleHelpAction,
		NewTask:             m.focusBoardCreateTaskAction(true),
		NewTaskWithTemplate: m.focusBoardCreateTaskAction(false),
		DeleteTask:          m.focusBoardDeleteTask,
		StatusChange:        m.focusBoardStatusChangeAction,
		MarkDone:            m.markDone,
		OpenURL:             m.focusBoardOpenURL,
		MoveToProject:       m.focusBoardMoveToProject,
		QuickNote:           m.focusBoardQuickNote,
	}
}

func (m *FocusBoardModel) focusBoardHasItem() bool {
	task, _ := m.getTaskRef()
	return task != nil
}

func focusBoardSortGroup(deps keymap.SortDeps) []keymap.Entry {
	cat := "Sort"
	return []keymap.Entry{
		keymap.Bind("1", "Sort within groups: priority", cat, deps.SortByPriority),
		keymap.Bind("2", "Sort within groups: due date", cat, deps.SortByDueDate),
		keymap.Bind("3", "Sort within groups: created", cat, deps.SortByCreated),
		keymap.Bind("4", "Sort within groups: status", cat, deps.SortByStatus),
		keymap.Bind("5", "Sort within groups: score", cat, deps.SortByScore),
		keymap.Bind("0", "Reset group sort", cat, deps.ResetSort),
	}
}

func (m *FocusBoardModel) focusBoardSortAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		refresh := func() {
			m.applyFilter()
			m.clampCursor()
		}
		handleSortKey(key, &m.sort, refresh, func(SortField) { refresh() })
		return m, nil
	}
}

func (m *FocusBoardModel) focusBoardTogglePreview() (tea.Model, tea.Cmd) {
	m.previewMode = !m.previewMode
	m.resetViewScrollOffset()
	return m, nil
}

func (m *FocusBoardModel) focusBoardCreateTaskAction(skipTemplatePicker bool) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		task, project := m.getTaskRef()
		if task == nil || project == nil {
			return m, nil
		}
		return m, func() tea.Msg {
			return CreateTaskMsg{
				ParentTask:         task,
				Project:            project,
				SkipTemplatePicker: skipTemplatePicker,
			}
		}
	}
}

func (m *FocusBoardModel) focusBoardDeleteTask() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *FocusBoardModel) focusBoardStatusChangeAction(status string) keymap.Action {
	s, ok := models.ParseTaskStatus(status)
	if !ok {
		return func() (tea.Model, tea.Cmd) { return m, nil }
	}
	if s == models.StatusDone {
		return m.markDone
	}
	return func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(m, s) }
}

func (m *FocusBoardModel) focusBoardOpenURL() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *FocusBoardModel) focusBoardMoveToProject() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *FocusBoardModel) focusBoardQuickNote() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *FocusBoardModel) focusBoardGoToProject() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return FocusBoardSelectMsg{Task: task, Project: project} }
}

func (m *FocusBoardModel) parentRowIndices() []int {
	var indices []int
	for i, row := range m.filtered {
		if row.Task != nil && row.TreePrefix == "" {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *FocusBoardModel) focusBoardGoToNextParent() tea.Cmd {
	old := m.cursor
	cmd := m.SectionGoToNext(m.parentRowIndices())
	m.clampCursor()
	if cmd != nil {
		return cmd
	}
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *FocusBoardModel) focusBoardGoToPrevParent() tea.Cmd {
	old := m.cursor
	cmd := m.SectionGoToPrev(m.parentRowIndices())
	m.clampCursor()
	if cmd != nil {
		return cmd
	}
	return flashIfJumped(old, m.cursor, &m.cursorFlash)
}
