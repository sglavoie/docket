package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for PriorityViewModel.
func (m *PriorityViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, m.priorityNavigationEntries()...)
	entries = append(entries, m.priorityGotoEntries()...)
	entries = append(entries, m.priorityPreviewEntries()...)
	entries = append(entries, buildStandardTaskActionEntries(m.priorityTaskActionDeps())...)
	entries = append(entries, m.priorityFilterEntries()...)
	entries = append(entries, m.priorityViewSpecificEntries()...)

	return &keymap.KeyMap{
		ViewName: "Priority",
		Entries:  entries,
	}
}

func (m *PriorityViewModel) priorityNavigationEntries() []keymap.Entry {
	return keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown:   m.priorityCursorDownAction,
		CursorUp:     m.priorityCursorUpAction,
		JumpToBottom: m.priorityJumpToBottomAction,
		PageUp:       m.priorityPageUpAction,
		PageDown:     m.priorityPageDownAction,
	})
}

func (m *PriorityViewModel) priorityCursorDownAction() (tea.Model, tea.Cmd) {
	if m.cursor < m.totalTasks-1 {
		m.cursor++
	}
	return m, nil
}

func (m *PriorityViewModel) priorityCursorUpAction() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.cursor--
	}
	return m, nil
}

func (m *PriorityViewModel) priorityJumpToBottomAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	if m.totalTasks > 0 {
		m.cursor = m.totalTasks - 1
	}
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *PriorityViewModel) priorityPageUpAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalTasks, -HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *PriorityViewModel) priorityPageDownAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalTasks, HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *PriorityViewModel) priorityGotoEntries() []keymap.Entry {
	return keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: m.priorityGoToProjectAction,
		Extra: []keymap.Entry{
			keymap.Bind("t", "Go to task in outliner", "Navigation", m.priorityGoToTaskAction),
			keymap.Bind("h", "Open task activity log", "Navigation", m.priorityOpenActivityAction),
		},
	})
}

func (m *PriorityViewModel) priorityGoToProjectAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	return m, func() tea.Msg { return PriorityViewSelectMsg{Task: task} }
}

func (m *PriorityViewModel) priorityGoToTaskAction() (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}
	return m, func() tea.Msg { return PriorityViewSelectMsg{Task: pt.Task} }
}

func (m *PriorityViewModel) priorityOpenActivityAction() (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}
	project := m.project
	return m, func() tea.Msg { return OpenTaskActivityMsg{Task: pt.Task, Project: project} }
}

func (m *PriorityViewModel) priorityPreviewEntries() []keymap.Entry {
	return []keymap.Entry{
		keymap.Prefix("f", "Preview", "View", []keymap.Entry{
			keymap.Bind("p", "Toggle inline preview", "View", m.priorityTogglePreviewAction),
		}),
	}
}

func (m *PriorityViewModel) priorityTogglePreviewAction() (tea.Model, tea.Cmd) {
	m.previewMode = !m.previewMode
	m.resetViewScrollOffset()
	return m, nil
}

func (m *PriorityViewModel) priorityTaskActionDeps() TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.priorityHasItem,
		ToggleHelp:       m.priorityToggleHelpAction,
		SelectedTaskRefs: m.prioritySelectedTaskRefs,
		DeleteTask:       m.priorityDeleteTaskAction,
		OpenURL:          m.priorityOpenURLAction,
		MoveToProject:    m.priorityMoveToProjectAction,
		QuickNote:        m.priorityQuickNoteAction,
		StateChange:      m.priorityStatusChangeAction,
		PriorityChange:   m.priorityPriorityChangeAction,
		DueDateSet:       m.priorityDueDateSetAction,
		DueDateClear:     m.priorityDueDateClearAction,
		SnoozeDate:       m.prioritySnoozeDateAction,
		SectionMove:      m.prioritySectionMoveAction,
	}
}

func (m *PriorityViewModel) priorityHasItem() bool {
	return m.getTaskAtCursor() != nil
}

func (m *PriorityViewModel) prioritySelectedTaskRefs() []TaskProjectRef {
	if !m.hasBulkSelection() {
		return nil
	}
	return m.getSelectedBulkItems()
}

func (m *PriorityViewModel) priorityToggleHelpAction() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *PriorityViewModel) priorityDeleteTaskAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *PriorityViewModel) priorityOpenURLAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *PriorityViewModel) priorityMoveToProjectAction() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *PriorityViewModel) priorityQuickNoteAction() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *PriorityViewModel) hasBulkSelection() bool {
	return m.multiSelectMode && m.SelectedCount() > 0
}

func (m *PriorityViewModel) priorityStatusChangeAction(state string) keymap.Action {
	s := models.WorkflowState(state)
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			if cmd := bulkListStateCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), s); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfChangeState(m, s)
	}
}

func (m *PriorityViewModel) priorityPriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			priority, _ := PriorityForPrefixKey(key)
			if cmd := bulkListPriorityCmd(m.bulk, m.getSelectedBulkItems(), priority); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfChangePriority(m, key)
	}
}

func (m *PriorityViewModel) priorityDueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			if cmd := bulkListDueDateCmd(m.bulk, m.getSelectedBulkItems(), days); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfSetQuickDueDate(m, days)
	}
}

func (m *PriorityViewModel) priorityDueDateClearAction() (tea.Model, tea.Cmd) {
	if m.hasBulkSelection() {
		if cmd := bulkListClearDueDateCmd(m.bulk, m.getSelectedBulkItems()); cmd != nil {
			return m, cmd
		}
		return m, nil
	}
	return m, wfClearDueDate(m)
}

func (m *PriorityViewModel) prioritySnoozeDateAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return m, nil
		}
		if m.hasBulkSelection() {
			if cmd := bulkListSnoozeCmd(m.bulk, m.getSelectedBulkItems(), snoozeDate); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m, wfQuickSnooze(m, snoozeDate)
	}
}

func (m *PriorityViewModel) prioritySectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			cmd, ok := bulkListSectionMoveCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), section)
			if !ok {
				m.setStatus(fmt.Sprintf("Unknown section %q", section))
			}
			return m, cmd
		}
		return m, wfSectionMove(m, section)
	}
}

func (m *PriorityViewModel) priorityFilterEntries() []keymap.Entry {
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      m.priorityFilterPrefixAction("#"),
		TogglePriority:    m.priorityFilterPrefixAction("p"),
		ToggleHideDimmed:  m.priorityFilterPrefixAction("h"),
		ToggleGitHubIssue: m.priorityFilterPrefixAction("i"),
		ToggleParked:      m.priorityFilterPrefixAction("k"),
		ClearFilters:      m.priorityFilterPrefixAction("x"),
		TypeKey:           m.priorityFilterKeyAction("Ft"),
		StateKey:          m.priorityFilterKeyAction("Fs"),
		DueDateKey:        m.priorityFilterKeyAction("Fd"),
		AttrDirectKey:     m.priorityFilterKeyAction("Fa"),
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewPriorityView} }
		},
		ScoreKey: m.priorityFilterKeyAction("FS"),
	})
	if len(filterEntries) > 0 && filterEntries[0].PrefixNode != nil {
		filterEntries[0].PrefixNode.Children = append(filterEntries[0].PrefixNode.Children,
			keymap.Bind("r", "Toggle actionable filter", "Filters", m.priorityToggleActionableFilterAction),
		)
	}
	return filterEntries
}

func (m *PriorityViewModel) priorityFilterPrefixAction(prefix string) keymap.Action {
	return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterPrefix(prefix) }
}

func (m *PriorityViewModel) priorityFilterKeyAction(prefix string) func(string) keymap.Action {
	return func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix(prefix, key) }
	}
}

func (m *PriorityViewModel) priorityToggleActionableFilterAction() (tea.Model, tea.Cmd) {
	m.filter.Actionable = !m.filter.Actionable
	m.loadActionableTasks()
	return m, nil
}

func (m *PriorityViewModel) priorityViewSpecificEntries() []keymap.Entry {
	entries := []keymap.Entry{
		keymap.Bind("enter", "View task details", "Actions", m.priorityOpenTaskDetailAction),
		keymap.Bind("tab", "Toggle waiting section", "View", m.priorityToggleWaitingAction),
		keymap.Bind("shift+right", "Next tab", "Navigation", m.priorityHorizontalTabAction(1)),
		keymap.Bind("shift+left", "Previous tab", "Navigation", m.priorityHorizontalTabAction(-1)),
		keymap.Bind("shift+down", "Enter sub-tab row focus", "Navigation", m.prioritySetSubTabFocusAction(true)),
		keymap.Bind("shift+up", "Exit sub-tab row focus", "Navigation", m.prioritySetSubTabFocusAction(false)),
		keymap.Bind("w", "Start suggested task", "Suggestion", m.priorityStartSuggestedAction),
		keymap.Bind("x", "Dismiss current suggestion", "Suggestion", m.priorityDismissSuggestionAction),
		keymap.Bind("A", "All tasks", "View", m.priorityAllTasksAction),
		keymap.Bind("/", "Search tasks", "Filter", m.prioritySearchAction),
		keymap.Bind("r", "Refresh (clears dismissed)", "View", m.priorityRefreshAction),
		keymap.Bind("p", "Close Priority view", "Views", m.priorityCloseViewAction),
	}
	entries = append(entries, m.priorityQuickPriorityEntries()...)
	entries = append(entries, m.priorityMultiSelectEntries()...)
	return entries
}

func (m *PriorityViewModel) priorityCloseViewAction() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg { return ClosePriorityViewMsg{} }
}

func (m *PriorityViewModel) priorityOpenTaskDetailAction() (tea.Model, tea.Cmd) {
	pt := m.getTaskAtCursor()
	if pt == nil {
		return m, nil
	}
	project := m.project
	return m, func() tea.Msg { return ShowTaskDetailMsg{Task: pt.Task, Project: project} }
}

func (m *PriorityViewModel) priorityToggleWaitingAction() (tea.Model, tea.Cmd) {
	m.waitingOnCollapsed = !m.waitingOnCollapsed
	m.resetViewScrollOffset()
	return m, nil
}

func (m *PriorityViewModel) priorityHorizontalTabAction(step int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.subTabRowFocused && len(m.subTabNames) > 1 {
			total := len(m.subTabNames) + 1
			CycleTab(&m.subTab, total, step)
		} else {
			m.tabs.Cycle(step)
			m.subTab = 0
			m.subTabRowFocused = false
		}
		m.recalculateNavigation()
		m.clampCursor()
		return m, nil
	}
}

func (m *PriorityViewModel) prioritySetSubTabFocusAction(focused bool) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if len(m.subTabNames) > 1 {
			m.subTabRowFocused = focused
		}
		return m, nil
	}
}

func (m *PriorityViewModel) priorityStartSuggestedAction() (tea.Model, tea.Cmd) {
	return m.startSuggestedTask()
}

func (m *PriorityViewModel) priorityDismissSuggestionAction() (tea.Model, tea.Cmd) {
	m.dismissSuggestion()
	m.resetViewScrollOffset()
	return m, nil
}

func (m *PriorityViewModel) priorityAllTasksAction() (tea.Model, tea.Cmd) {
	if m.multiSelectMode {
		m.deselectAll()
		return m, nil
	}
	return m, func() tea.Msg { return ShowAllTasksMsg{} }
}

func (m *PriorityViewModel) prioritySearchAction() (tea.Model, tea.Cmd) {
	return m, m.ActivateSearch()
}

func (m *PriorityViewModel) priorityRefreshAction() (tea.Model, tea.Cmd) {
	m.dismissedIDs = make(map[string]bool)
	m.loadActionableTasks()
	m.clampCursor()
	m.setStatus("Refreshed")
	return m, nil
}

func (m *PriorityViewModel) priorityQuickPriorityEntries() []keymap.Entry {
	keys := []string{"1", "2", "3", "4", "5"}
	entries := make([]keymap.Entry, 0, len(keys))
	for _, key := range keys {
		priorityKey := key
		entries = append(entries, keymap.Bind(priorityKey, "Set priority "+priorityKey, "Actions", func() (tea.Model, tea.Cmd) {
			return m.changeTaskPriority(int(priorityKey[0] - '0'))
		}, keymap.AsHidden()))
	}
	return entries
}

func (m *PriorityViewModel) priorityMultiSelectEntries() []keymap.Entry {
	return []keymap.Entry{
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
	}
}
