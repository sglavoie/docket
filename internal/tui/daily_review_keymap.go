package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for DailyReviewModel.
func (m *DailyReviewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, m.dailyReviewNavigationEntries()...)
	entries = append(entries, m.dailyReviewTaskActionEntries()...)
	entries = append(entries, m.dailyReviewFilterEntries()...)
	entries = append(entries, m.dailyReviewDirectEntries()...)

	return &keymap.KeyMap{
		ViewName: "Daily Review",
		Entries:  entries,
	}
}

func (m *DailyReviewModel) dailyReviewNavigationEntries() []keymap.Entry {
	focusSection := func(s DailyReviewSection) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			return m.toggleSectionFocus(s)
		}
	}

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
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return DailyReviewSelectMsg{Task: task, Project: project} }
		},
		Extra: []keymap.Entry{
			keymap.Bind("d", "Focus Overdue section", "Sections", focusSection(SectionOverdue)),
			keymap.Bind("t", "Focus Today section", "Sections", focusSection(SectionDueToday)),
			keymap.Bind("s", "Focus Due Soon section", "Sections", focusSection(SectionDueSoon)),
			keymap.Bind("l", "Focus Due Later section", "Sections", focusSection(SectionDueLater)),
			keymap.Bind("f", "Focus Future section", "Sections", focusSection(SectionFuture)),
			keymap.Bind("r", "Focus In Review section", "Sections", focusSection(SectionInReview)),
			keymap.Bind("S", "Focus On Standby section", "Sections", focusSection(SectionOnStandby)),
			keymap.Bind("o", "Focus Old section", "Sections", focusSection(SectionOld)),
		},
	})...)

	entries = append(entries, keymap.Prefix("f", "View", "View", []keymap.Entry{
		keymap.Bind("a", "Toggle section fold", "Folds", func() (tea.Model, tea.Cmd) {
			return m.toggleCurrentSection()
		}),
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		}),
	}))

	return entries
}

func (m *DailyReviewModel) dailyReviewTaskActionEntries() []keymap.Entry {
	return buildReviewTaskActionEntries(m.dailyReviewTaskActionDeps())
}

func (m *DailyReviewModel) dailyReviewTaskActionDeps() TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.dailyReviewHasItem,
		ToggleHelp:       m.dailyReviewToggleHelp,
		SelectedTaskRefs: m.dailyReviewSelectedTaskRefs,
		DeleteTask:       m.dailyReviewDeleteTask,
		StateChange:      m.dailyReviewStatusChangeAction,
		PriorityChange:   m.dailyReviewPriorityChangeAction,
		DueDateSet:       m.dailyReviewDueDateSetAction,
		DueDateClear:     m.dailyReviewClearDueDate,
		SnoozeDate:       m.dailyReviewSnoozeDateAction,
		SectionMove:      m.dailyReviewSectionMoveAction,
		OpenURL:          m.dailyReviewOpenURL,
		MoveToProject:    m.dailyReviewMoveToProject,
		QuickNote:        m.dailyReviewQuickNote,
	}
}

func (m *DailyReviewModel) dailyReviewHasItem() bool { return m.getTaskAtCursor() != nil }

func (m *DailyReviewModel) dailyReviewSelectedTaskRefs() []TaskProjectRef {
	if !m.multiSelectMode || m.SelectedCount() == 0 {
		return nil
	}
	return m.getSelectedBulkItems()
}

func (m *DailyReviewModel) dailyReviewToggleHelp() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *DailyReviewModel) dailyReviewDeleteTask() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *DailyReviewModel) dailyReviewStatusChangeAction(state string) keymap.Action {
	s := models.WorkflowState(state)
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			if cmd := bulkListStateCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), s); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return reviewChangeState(m, s)
	}
}

func (m *DailyReviewModel) dailyReviewPriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			p, _ := PriorityForPrefixKey(key)
			if cmd := bulkListPriorityCmd(m.bulk, m.getSelectedBulkItems(), p); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return reviewChangePriority(m, key)
	}
}

func (m *DailyReviewModel) dailyReviewDueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			if cmd := bulkListDueDateCmd(m.bulk, m.getSelectedBulkItems(), days); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return reviewSetQuickDueDate(m, days)
	}
}

func (m *DailyReviewModel) dailyReviewClearDueDate() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		if cmd := bulkListClearDueDateCmd(m.bulk, m.getSelectedBulkItems()); cmd != nil {
			return m, cmd
		}
		return m, nil
	}
	return reviewClearDueDate(m)
}

func (m *DailyReviewModel) dailyReviewSnoozeDateAction(key string) keymap.Action {
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
		return reviewQuickSnooze(m, snoozeDate)
	}
}

func (m *DailyReviewModel) dailyReviewSectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			cmd, ok := bulkListSectionMoveCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), section)
			if !ok {
				m.setStatus(fmt.Sprintf("Unknown section %q", section))
			}
			return m, cmd
		}
		return reviewSectionMove(m, section)
	}
}

func (m *DailyReviewModel) dailyReviewOpenURL() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *DailyReviewModel) dailyReviewMoveToProject() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *DailyReviewModel) dailyReviewQuickNote() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *DailyReviewModel) dailyReviewFilterEntries() []keymap.Entry {
	return keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("#") },
		TogglePriority:    func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("p") },
		ToggleHideDimmed:  func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("h") },
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("i") },
		ToggleParked:      func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("k") },
		ClearFilters:      func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Ft", key) }
		},
		StateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Fd", key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewDailyReview} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("FS", key) }
		},
	})
}

func (m *DailyReviewModel) dailyReviewDirectEntries() []keymap.Entry {
	entries := []keymap.Entry{
		keymap.Bind("enter", "Navigate to task in project", "Navigation", func() (tea.Model, tea.Cmd) {
			dt := m.getTaskAtCursor()
			if dt == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return DailyReviewSelectMsg{Task: dt.Task, Project: dt.Project}
			}
		}),
		keymap.Bind("tab", "Collapse/expand section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.toggleCurrentSection()
		}),
		keymap.Bind("n", "Next section (show all)", "Navigation", func() (tea.Model, tea.Cmd) {
			m.focusedSection = nil
			m.calculateNavigation()
			return m.goToNextSection()
		}),
		keymap.Bind("N", "Previous section (show all)", "Navigation", func() (tea.Model, tea.Cmd) {
			m.focusedSection = nil
			m.calculateNavigation()
			return m.goToPrevSection()
		}),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+right", &m.tabs, func() {
				m.focusedSection = nil
				m.scrollOffset = 0
				m.calculateNavigation()
				m.clampCursor()
			})
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+left", &m.tabs, func() {
				m.focusedSection = nil
				m.scrollOffset = 0
				m.calculateNavigation()
				m.clampCursor()
			})
			return m, nil
		}),
		keymap.Bind("R", "Toggle review completion", "View", func() (tea.Model, tea.Cmd) {
			if m.reviewCompleted {
				m.reviewCompleted = false
				return m, func() tea.Msg { return ResetDailyReviewMsg{} }
			}
			m.reviewCompleted = true
			m.completedAt = time.Now()
			return m, func() tea.Msg { return MarkDailyReviewDoneMsg{} }
		}),
	}

	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return reviewChangePriority(m, key)
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
		keymap.Bind("ctrl+f", "Switch to Weekly Review", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowWeeklyReviewMsg{} }
		}),
	)

	return entries
}
