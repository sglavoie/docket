package tui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for FocusViewModel.
func (m *FocusViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, m.focusNavigationEntries()...)
	entries = append(entries, m.focusTaskActionEntries()...)
	entries = append(entries, m.focusFilterEntries()...)
	entries = append(entries, m.focusDirectEntries()...)

	return &keymap.KeyMap{
		ViewName: "Backlog",
		Entries:  entries,
	}
}

func (m *FocusViewModel) focusNavigationEntries() []keymap.Entry {
	entries := keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.filtered)-1 {
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
	})

	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return FocusViewSelectMsg{Task: task, Project: project} }
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

func (m *FocusViewModel) focusTaskActionEntries() []keymap.Entry {
	return buildStandardTaskActionEntries(m.focusTaskActionDeps())
}

func (m *FocusViewModel) focusTaskActionDeps() TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.focusHasItem,
		ToggleHelp:       m.focusToggleHelp,
		SelectedTaskRefs: m.focusSelectedTaskRefs,
		DeleteTask:       m.focusDeleteTask,
		StateChange:      m.focusStatusChangeAction,
		PriorityChange:   m.focusPriorityChangeAction,
		DueDateSet:       m.focusDueDateSetAction,
		DueDateClear:     m.focusClearDueDate,
		SnoozeDate:       m.focusSnoozeDateAction,
		SectionMove:      m.focusSectionMoveAction,
		MarkDone:         m.focusMarkDone,
		OpenURL:          m.focusOpenURL,
		MoveToProject:    m.focusMoveToProject,
		QuickNote:        m.focusQuickNote,
	}
}

func (m *FocusViewModel) focusHasItem() bool { return m.getTaskAtCursor() != nil }

func (m *FocusViewModel) focusSelectedTaskRefs() []TaskProjectRef {
	if !m.multiSelectMode || m.SelectedCount() == 0 {
		return nil
	}
	return m.getSelectedBulkItems()
}

func (m *FocusViewModel) focusToggleHelp() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *FocusViewModel) focusDeleteTask() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *FocusViewModel) focusStatusChangeAction(state string) keymap.Action {
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

func (m *FocusViewModel) focusPriorityChangeAction(key string) keymap.Action {
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

func (m *FocusViewModel) focusDueDateSetAction(days int) keymap.Action {
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

func (m *FocusViewModel) focusClearDueDate() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		if cmd := bulkListClearDueDateCmd(m.bulk, m.getSelectedBulkItems()); cmd != nil {
			return m, cmd
		}
		return m, nil
	}
	return m, wfClearDueDate(m)
}

func (m *FocusViewModel) focusSnoozeDateAction(key string) keymap.Action {
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

func (m *FocusViewModel) focusSectionMoveAction(section string) keymap.Action {
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

func (m *FocusViewModel) focusMarkDone() (tea.Model, tea.Cmd) {
	if m.tabs.Active == focusTabSuggested {
		m.showDismissed = !m.showDismissed
		m.recomputeSuggested()
		m.clampCursor()
		return m, nil
	}
	return m, wfChangeState(m, models.StateDone)
}

func (m *FocusViewModel) focusOpenURL() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *FocusViewModel) focusMoveToProject() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *FocusViewModel) focusQuickNote() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *FocusViewModel) focusFilterEntries() []keymap.Entry {
	return keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("#") },
		TogglePriority:    func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("p") },
		ToggleHideDimmed:  func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("h") },
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("i") },
		ToggleParked:      func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("k") },
		ClearFilters:      func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("Ft", key)
			}
		},
		StateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("Fs", key)
			}
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("Fd", key)
			}
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix(prefix, key)
			}
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewFocusView} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("FS", key)
			}
		},
	})
}

func (m *FocusViewModel) focusDirectEntries() []keymap.Entry {
	entries := []keymap.Entry{
		keymap.Bind("enter", "Navigate to task in project", "Navigation", func() (tea.Model, tea.Cmd) {
			ft := m.getTaskAtCursor()
			if ft == nil {
				return m, nil
			}
			return m, func() tea.Msg { return FocusViewSelectMsg{Task: ft.Task, Project: ft.Project} }
		}),
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleTabNavigation(1)
		}),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleTabNavigation(1)
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleTabNavigation(-1)
		}, keymap.AsHidden()),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleTabNavigation(-1)
		}, keymap.AsHidden()),
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
		keymap.Bind("J", "Move item down in queue", "Queue", func() (tea.Model, tea.Cmd) {
			return m.handleQueueReorder(1)
		}),
		keymap.Bind("K", "Move item up in queue", "Queue", func() (tea.Model, tea.Cmd) {
			return m.handleQueueReorder(-1)
		}),
		keymap.Bind("x", "Dismiss or remove from queue", "Actions", func() (tea.Model, tea.Cmd) {
			switch m.tabs.Active {
			case focusTabQueue:
				m.removeFromQueue()
			case focusTabSuggested:
				m.toggleDismiss()
			}
			return m, nil
		}),
		keymap.Bind("r", "Refresh", "View", func() (tea.Model, tea.Cmd) {
			m.refreshViewState()
			return m, nil
		}),
		keymap.Bind("n", "Next parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToNext(m.parentRowIndices())
		}),
		keymap.Bind("N", "Previous parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToPrev(m.parentRowIndices())
		}),
	}

	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return m, wfChangePriority(m, key)
			}, keymap.AsHidden()),
		)
	}

	entries = append(entries,
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
		keymap.Bind("ctrl+t", "Switch to Today (Work mode)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusBoardMsg{} }
		}),
	)

	return entries
}
