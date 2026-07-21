package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for AllTasksViewModel.
func (m *AllTasksViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, m.allTasksNavigationEntries()...)
	entries = append(entries, m.allTasksViewEntries()...)
	entries = append(entries, buildStandardTaskActionEntries(m.allTasksTaskKeymapDeps())...)
	entries = append(entries, m.allTasksFilterEntries()...)
	entries = append(entries, m.allTasksSortEntries()...)
	entries = append(entries, m.allTasksTabEntries()...)
	entries = append(entries, m.allTasksMultiSelectEntries()...)
	entries = append(entries, m.allTasksExportEntries()...)

	return &keymap.KeyMap{
		ViewName: "All Tasks",
		Entries:  entries,
	}
}

func (m *AllTasksViewModel) allTasksNavigationEntries() []keymap.Entry {
	entries := keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown:   m.allTasksCursorDown,
		CursorUp:     m.allTasksCursorUp,
		JumpToBottom: m.allTasksJumpToBottom,
		PageUp:       m.allTasksPageUp,
		PageDown:     m.allTasksPageDown,
	})
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   m.allTasksJumpToTop,
		GoToProject: m.allTasksGoToProject,
	})...)
	entries = append(entries,
		keymap.Bind("enter", "View task details", "Actions", m.allTasksShowTaskDetail),
		keymap.Bind("n", "Next group header", "Navigation", m.goToNextGroup),
		keymap.Bind("N", "Previous group header", "Navigation", m.goToPrevGroup),
	)
	return entries
}

func (m *AllTasksViewModel) allTasksViewEntries() []keymap.Entry {
	return []keymap.Entry{
		keymap.Prefix("f", "View", "View", []keymap.Entry{
			keymap.Bind("a", "Toggle group fold", "Folds", m.allTasksToggleGroupCollapse),
			keymap.Bind("p", "Toggle inline preview", "View", m.allTasksTogglePreview),
		}),
		keymap.Bind("ctrl+g", "Cycle grouping mode", "View", m.allTasksCycleGrouping),
		keymap.Bind("tab", "Collapse/expand group", "View", m.allTasksToggleGroupCollapse),
	}
}

func (m *AllTasksViewModel) allTasksTaskKeymapDeps() TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.allTasksHasItem,
		ToggleHelp:       m.allTasksToggleHelp,
		SelectedTaskRefs: m.allTasksSelectedTaskRefs,
		DeleteTask:       m.allTasksDeleteTask,
		OpenURL:          m.allTasksOpenURL,
		MoveToProject:    m.allTasksMoveToProject,
		QuickNote:        m.allTasksQuickNote,
		StateChange:      m.allTasksStateChangeAction,
		PriorityChange:   m.allTasksPriorityChangeAction,
		DueDateSet:       m.allTasksDueDateSetAction,
		DueDateClear:     m.allTasksClearDueDate,
		SectionMove:      m.allTasksSectionMoveAction,
		MarkDone:         m.allTasksMarkDone,
	}
}

func (m *AllTasksViewModel) allTasksFilterEntries() []keymap.Entry {
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      m.allTasksFilterByTags,
		TogglePriority:    m.allTasksTogglePriorityFilter,
		ToggleHideDimmed:  m.allTasksToggleHideDimmedFilter,
		ToggleGitHubIssue: m.allTasksToggleGitHubIssueFilter,
		ToggleParked:      m.allTasksToggleParkedFilter,
		ClearFilters:      m.allTasksClearFilters,
		TypeKey:           m.allTasksFilterTypeKey,
		StateKey:          m.allTasksFilterStateKey,
		DueDateKey:        nil,
		OpenAttrPicker:    m.allTasksOpenAttrPicker,
		ScoreKey:          m.allTasksFilterScoreKey,
	})
	if len(filterEntries) == 0 || filterEntries[0].PrefixNode == nil {
		return filterEntries
	}
	filterEntries[0].PrefixNode.Children = append(filterEntries[0].PrefixNode.Children,
		keymap.Bind("d", "Cycle due date filter", "Filters", m.allTasksCycleDueDateFilter),
		keymap.Bind("P", "Toggle project filter", "Filters", m.allTasksToggleProjectFilter),
	)
	return filterEntries
}

func (m *AllTasksViewModel) allTasksSortEntries() []keymap.Entry {
	return keymap.SortGroup(keymap.SortDeps{
		SortByPriority: m.allTasksSortAction("1"),
		SortByDueDate:  m.allTasksSortAction("2"),
		SortByCreated:  m.allTasksSortAction("3"),
		SortByStatus:   m.allTasksSortAction("4"),
		SortByScore:    m.allTasksSortAction("5"),
		ResetSort:      m.allTasksSortAction("0"),
	})
}

func (m *AllTasksViewModel) allTasksTabEntries() []keymap.Entry {
	return []keymap.Entry{
		keymap.Bind("shift+left", "Previous tab", "Navigation", m.allTasksCycleTabAction(-1), keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", m.allTasksCycleTabAction(1), keymap.AsHidden()),
	}
}

func (m *AllTasksViewModel) allTasksMultiSelectEntries() []keymap.Entry {
	return []keymap.Entry{
		keymap.Bind("v", "Toggle multi-select", "Multi-select", m.allTasksToggleMultiSelect),
		keymap.Bind("space", "Toggle selection (multi-select) / toggle peek", "Multi-select", m.allTasksToggleSelection, keymap.AsHidden()),
		keymap.Bind("a", "Select all visible", "Multi-select", m.allTasksSelectAllVisible, keymap.AsHidden()),
		keymap.Bind("A", "Deselect all", "Multi-select", m.allTasksDeselectAll, keymap.AsHidden()),
	}
}

func (m *AllTasksViewModel) allTasksExportEntries() []keymap.Entry {
	return []keymap.Entry{
		keymap.Bind("ctrl+e", "Export task's project to Markdown", "Actions", m.allTasksExportProject),
	}
}

func (m *AllTasksViewModel) allTasksHasItem() bool { return m.getTaskAtCursor() != nil }

func (m *AllTasksViewModel) allTasksSelectedTaskRefs() []TaskProjectRef {
	if !m.multiSelectMode || m.SelectedCount() == 0 {
		return nil
	}
	return m.getSelectedBulkItems()
}

func (m *AllTasksViewModel) allTasksCursorDown() (tea.Model, tea.Cmd) {
	if m.cursor < m.totalNavigableItems()-1 {
		m.cursor++
	}
	return m, nil
}

func (m *AllTasksViewModel) allTasksCursorUp() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.cursor--
	}
	return m, nil
}

func (m *AllTasksViewModel) allTasksJumpToBottom() (tea.Model, tea.Cmd) {
	old := m.cursor
	if n := m.totalNavigableItems(); n > 0 {
		m.cursor = n - 1
	}
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *AllTasksViewModel) allTasksPageUp() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalNavigableItems(), -HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *AllTasksViewModel) allTasksPageDown() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalNavigableItems(), HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *AllTasksViewModel) allTasksJumpToTop() (tea.Model, tea.Cmd) {
	return m, m.JumpToTop()
}

func (m *AllTasksViewModel) allTasksGoToProject() (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}
	return m, func() tea.Msg { return AllTasksSelectMsg{Task: item.Task, Project: item.Project} }
}

func (m *AllTasksViewModel) allTasksShowTaskDetail() (tea.Model, tea.Cmd) {
	item := m.getTaskAtCursor()
	if item == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
	}
}

func (m *AllTasksViewModel) allTasksToggleGroupCollapse() (tea.Model, tea.Cmd) {
	m.toggleGroupCollapse()
	return m, nil
}

func (m *AllTasksViewModel) allTasksTogglePreview() (tea.Model, tea.Cmd) {
	m.previewMode = !m.previewMode
	return m, nil
}

func (m *AllTasksViewModel) allTasksCycleGrouping() (tea.Model, tea.Cmd) {
	m.cycleGroupBy()
	return m, nil
}

func (m *AllTasksViewModel) allTasksToggleHelp() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *AllTasksViewModel) allTasksDeleteTask() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *AllTasksViewModel) allTasksOpenURL() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *AllTasksViewModel) allTasksMoveToProject() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *AllTasksViewModel) allTasksQuickNote() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *AllTasksViewModel) allTasksStateChangeAction(state string) keymap.Action {
	s := models.WorkflowState(state)
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			return m.handleBulkStateChange(s)
		}
		cmd := wfChangeState(m, s)
		if s == models.StateDone {
			m.setCompletionFlash()
			return m, tea.Batch(cmd, completionFlashScheduler())
		}
		m.setMutationFlash(stateMutationFlashBg(s))
		return m, tea.Batch(cmd, mutationFlashTimeout())
	}
}

func (m *AllTasksViewModel) allTasksPriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		p, _ := PriorityForPrefixKey(key)
		if m.multiSelectMode && m.SelectedCount() > 0 {
			return m.handleBulkPriorityChange(p)
		}
		cmd := wfChangePriority(m, key)
		m.setMutationFlash(priorityMutationFlashBg(p))
		return m, tea.Batch(cmd, mutationFlashTimeout())
	}
}

func (m *AllTasksViewModel) allTasksDueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			return m.handleBulkDueDateChange(days)
		}
		return m, wfSetQuickDueDate(m, days)
	}
}

func (m *AllTasksViewModel) allTasksClearDueDate() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		return m.handleBulkClearDueDate()
	}
	return m, wfClearDueDate(m)
}

func (m *AllTasksViewModel) allTasksSectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			return m.handleBulkSectionMove(section)
		}
		return m, wfSectionMove(m, section)
	}
}

func (m *AllTasksViewModel) allTasksMarkDone() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		return m.handleBulkStateChange(models.StateDone)
	}
	cmd := wfChangeState(m, models.StateDone)
	m.setCompletionFlash()
	return m, tea.Batch(cmd, completionFlashScheduler())
}

func (m *AllTasksViewModel) allTasksFilterByTags() (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterPrefix("#")
}

func (m *AllTasksViewModel) allTasksTogglePriorityFilter() (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterPrefix("p")
}

func (m *AllTasksViewModel) allTasksToggleHideDimmedFilter() (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterPrefix("h")
}

func (m *AllTasksViewModel) allTasksToggleGitHubIssueFilter() (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterPrefix("i")
}

func (m *AllTasksViewModel) allTasksToggleParkedFilter() (tea.Model, tea.Cmd) {
	return m.handleAllTasksFilterPrefix("k")
}

func (m *AllTasksViewModel) allTasksClearFilters() (tea.Model, tea.Cmd) {
	clearCommonFilters(&m.filter)
	m.projectFilter = make(map[string]bool)
	m.applyFiltersAndGroup()
	m.clampCursor()
	return m, nil
}

func (m *AllTasksViewModel) allTasksFilterTypeKey(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		return m.handleAllTasksFilterSubPrefix("Ft", key)
	}
}

func (m *AllTasksViewModel) allTasksFilterStateKey(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		return m.handleAllTasksFilterSubPrefix("Fs", key)
	}
}

func (m *AllTasksViewModel) allTasksOpenAttrPicker() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewAllTasks} }
}

func (m *AllTasksViewModel) allTasksFilterScoreKey(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		return m.handleAllTasksFilterSubPrefix("FS", key)
	}
}

func (m *AllTasksViewModel) allTasksCycleDueDateFilter() (tea.Model, tea.Cmd) {
	m.filter.DueDate = (m.filter.DueDate + 1) % (DueDateNone + 1)
	m.applyFiltersAndGroup()
	m.clampCursor()
	return m, nil
}

func (m *AllTasksViewModel) allTasksToggleProjectFilter() (tea.Model, tea.Cmd) {
	m.toggleProjectFilter()
	return m, nil
}

func (m *AllTasksViewModel) allTasksSortAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		handleSortKey(key, &m.sort, func() {
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.setStatus("Sort reset")
		}, func(field SortField) {
			m.applyFiltersAndGroup()
			m.clampCursor()
			m.setStatus(fmt.Sprintf("Sort: %s %s", field, m.sortDirection()))
		})
		return m, nil
	}
}

func (m *AllTasksViewModel) cycleAllTasksTab(delta int) {
	total := len(m.allTabEntries)
	if total <= 1 {
		return
	}
	m.activeTabIdx = ((m.activeTabIdx+delta)%total + total) % total
	m.tabs.Active = min(m.activeTabIdx, len(m.tabEntries)-1)
	if m.activeTabIdx >= overflowTabCap-1 && len(m.allTabEntries) > overflowTabCap {
		m.overflowPickerCursor = m.activeTabIdx - (overflowTabCap - 1)
		m.overflowPickerOpen = true
	} else {
		m.overflowPickerOpen = false
	}
	if m.activeTabIncludesCompleted() != m.loadedWithCompleted {
		m.loadAllTasks()
	} else {
		m.applyFiltersAndGroup()
	}
	m.clampCursor()
}

func (m *AllTasksViewModel) allTasksCycleTabAction(delta int) func() (tea.Model, tea.Cmd) {
	return func() (tea.Model, tea.Cmd) {
		m.cycleAllTasksTab(delta)
		return m, nil
	}
}

func (m *AllTasksViewModel) allTasksToggleMultiSelect() (tea.Model, tea.Cmd) {
	m.toggleMultiSelectMode()
	return m, nil
}

func (m *AllTasksViewModel) allTasksToggleSelection() (tea.Model, tea.Cmd) {
	if m.multiSelectMode {
		m.toggleTaskSelection()
	}
	return m, nil
}

func (m *AllTasksViewModel) allTasksSelectAllVisible() (tea.Model, tea.Cmd) {
	if m.multiSelectMode {
		m.selectAllVisible()
	}
	return m, nil
}

func (m *AllTasksViewModel) allTasksDeselectAll() (tea.Model, tea.Cmd) {
	if m.multiSelectMode {
		m.deselectAll()
	}
	return m, nil
}

func (m *AllTasksViewModel) allTasksExportProject() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg { return ExportProjectMsg{} }
}
