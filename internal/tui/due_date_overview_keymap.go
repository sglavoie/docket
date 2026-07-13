package tui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for DueDateOverviewModel.
func (m *DueDateOverviewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, m.dueDateNavigationEntries()...)
	entries = append(entries, m.dueDateTaskActionEntries()...)
	entries = append(entries, m.dueDateFilterEntries()...)
	entries = append(entries, m.dueDateDirectEntries()...)

	return &keymap.KeyMap{
		ViewName: "Due Dates",
		Entries:  entries,
	}
}

func (m *DueDateOverviewModel) dueDateNavigationEntries() []keymap.Entry {
	entries := keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < m.totalTasks-1 {
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
			if m.totalTasks > 0 {
				m.cursor = m.totalTasks - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, m.totalTasks, -HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, m.totalTasks, HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})

	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			dt := m.getTaskAtCursor()
			if dt == nil {
				return m, nil
			}
			return m, func() tea.Msg { return DueDateOverviewSelectMsg{Task: dt.Task, Project: dt.Project} }
		},
	})...)

	entries = append(entries, keymap.Prefix("f", "Preview", "View", []keymap.Entry{
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			m.resetViewScrollOffset()
			return m, nil
		}),
	}))

	return entries
}

func (m *DueDateOverviewModel) dueDateTaskActionEntries() []keymap.Entry {
	return buildStandardTaskActionEntries(m.dueDateTaskActionDeps())
}

func (m *DueDateOverviewModel) dueDateTaskActionDeps() TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.dueDateHasItem,
		ToggleHelp:       m.dueDateToggleHelp,
		SelectedTaskRefs: m.dueDateSelectedTaskRefs,
		DeleteTask:       m.dueDateDeleteTask,
		OpenURL:          m.dueDateOpenURL,
		MoveToProject:    m.dueDateMoveToProject,
		QuickNote:        m.dueDateQuickNote,
		StateChange:      m.dueDateStatusChangeAction,
		PriorityChange:   m.dueDatePriorityChangeAction,
		DueDateSet:       m.dueDateSetAction,
		DueDateClear:     m.dueDateClearDueDate,
		SnoozeDate:       m.dueDateSnoozeDateAction,
		SectionMove:      m.dueDateSectionMoveAction,
	}
}

func (m *DueDateOverviewModel) dueDateHasItem() bool { return m.getTaskAtCursor() != nil }

func (m *DueDateOverviewModel) dueDateSelectedTaskRefs() []TaskProjectRef {
	if !m.multiSelectMode || m.SelectedCount() == 0 {
		return nil
	}
	return m.getSelectedBulkItems()
}

func (m *DueDateOverviewModel) dueDateToggleHelp() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *DueDateOverviewModel) dueDateDeleteTask() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *DueDateOverviewModel) dueDateOpenURL() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *DueDateOverviewModel) dueDateMoveToProject() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *DueDateOverviewModel) dueDateQuickNote() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *DueDateOverviewModel) dueDateStatusChangeAction(state string) keymap.Action {
	s := models.WorkflowState(state)
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			if cmd := bulkListStateCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), s); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfChangeState(m, s)
	}
}

func (m *DueDateOverviewModel) dueDatePriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			p, _ := PriorityForPrefixKey(key)
			if cmd := bulkListPriorityCmd(m.bulk, m.getSelectedBulkItems(), p); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfChangePriority(m, key)
	}
}

func (m *DueDateOverviewModel) dueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			if cmd := bulkListDueDateCmd(m.bulk, m.getSelectedBulkItems(), days); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfSetQuickDueDate(m, days)
	}
}

func (m *DueDateOverviewModel) dueDateClearDueDate() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		if cmd := bulkListClearDueDateCmd(m.bulk, m.getSelectedBulkItems()); cmd != nil {
			return m, cmd
		}
		return m, nil
	}
	return m, wfClearDueDate(m)
}

func (m *DueDateOverviewModel) dueDateSnoozeDateAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return m, nil
		}
		if m.multiSelectMode && m.SelectedCount() > 0 {
			if cmd := bulkListSnoozeCmd(m.bulk, m.getSelectedBulkItems(), snoozeDate); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfQuickSnooze(m, snoozeDate)
	}
}

func (m *DueDateOverviewModel) dueDateSectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			cmd, ok := bulkListSectionMoveCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), section)
			if !ok {
				m.setStatus(fmt.Sprintf("Unknown section %q", section))
			}
			return m, cmd
		}
		return m, wfSectionMove(m, section)
	}
}

func (m *DueDateOverviewModel) dueDateFilterEntries() []keymap.Entry {
	return keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      func() (tea.Model, tea.Cmd) { return m.handleDueDateFilterPrefix("#") },
		TogglePriority:    func() (tea.Model, tea.Cmd) { return m.handleDueDateFilterPrefix("p") },
		ToggleHideDimmed:  func() (tea.Model, tea.Cmd) { return m.handleDueDateFilterPrefix("h") },
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) { return m.handleDueDateFilterPrefix("i") },
		ToggleParked:      func() (tea.Model, tea.Cmd) { return m.handleDueDateFilterPrefix("k") },
		ClearFilters:      func() (tea.Model, tea.Cmd) { return m.handleDueDateFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleDueDateFilterSubPrefix("Ft", key)
			}
		},
		StateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleDueDateFilterSubPrefix("Fs", key)
			}
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleDueDateFilterSubPrefix("Fd", key)
			}
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleDueDateFilterSubPrefix("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleDueDateFilterSubPrefix(prefix, key)
			}
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewDueDateOverview} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleDueDateFilterSubPrefix("FS", key)
			}
		},
	})
}

func (m *DueDateOverviewModel) dueDateDirectEntries() []keymap.Entry {
	entries := []keymap.Entry{
		keymap.Bind("enter", "Navigate to task in project", "Navigation", func() (tea.Model, tea.Cmd) {
			dt := m.getTaskAtCursor()
			if dt == nil {
				return m, nil
			}
			return m, func() tea.Msg { return DueDateOverviewSelectMsg{Task: dt.Task, Project: dt.Project} }
		}),
	}

	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return m.changeTaskPriority(key)
			}, keymap.AsHidden()),
		)
	}

	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyTabFilter()
			return m, nil
		}),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyTabFilter()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyTabFilter()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyTabFilter()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("v", "Toggle multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection (multi-select) / toggle peek", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.toggleTaskSelection()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("a", "Select all visible", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.selectAllVisible()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("A", "Deselect all", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		}, keymap.AsHidden()),
	)

	return entries
}
