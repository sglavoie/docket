package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for PriorityViewModel.
func (m *PriorityViewModel) buildKeyMap() *keymap.KeyMap {
	stages := m.priorityStages()
	var entries []keymap.Entry
	entries = append(entries, m.priorityNavigationEntries()...)
	entries = append(entries, m.priorityGotoEntries()...)
	entries = append(entries, m.priorityPreviewEntries()...)
	entries = append(entries, buildStandardTaskActionEntries(m.priorityTaskActionDeps(stages))...)
	entries = append(entries, m.priorityFilterEntries(stages)...)
	entries = append(entries, m.priorityViewSpecificEntries()...)

	return &keymap.KeyMap{
		ViewName: "Priority",
		Entries:  entries,
	}
}

func (m *PriorityViewModel) priorityStages() []string {
	if m.config == nil {
		return nil
	}
	return m.config.Stages
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
			keymap.Bind("t", "Go to task in project tree", "Navigation", m.priorityGoToTaskAction),
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

func (m *PriorityViewModel) priorityTaskActionDeps(stages []string) TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.priorityHasItem,
		Stages:           stages,
		ToggleHelp:       m.priorityToggleHelpAction,
		SelectedTaskRefs: m.prioritySelectedTaskRefs,
		DeleteTask:       m.priorityDeleteTaskAction,
		OpenURL:          m.priorityOpenURLAction,
		MoveToProject:    m.priorityMoveToProjectAction,
		QuickNote:        m.priorityQuickNoteAction,
		StatusChange:     m.priorityStatusChangeAction,
		PriorityChange:   m.priorityPriorityChangeAction,
		DueDateSet:       m.priorityDueDateSetAction,
		DueDateClear:     m.priorityDueDateClearAction,
		SnoozeDate:       m.prioritySnoozeDateAction,
		SectionMove:      m.prioritySectionMoveAction,
		StageChange:      m.priorityStageChangeAction,
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

func (m *PriorityViewModel) runBulkAction(
	action string,
	zeroMsg string,
	successMsg func(int) string,
	mutate func(project *models.Project, taskID string) bool,
) (tea.Model, tea.Cmd) {
	items := m.getSelectedBulkItems()
	count, err := taskOpsApplyBulk(m, items, action, mutate)
	m.ExitMultiSelect()

	switch {
	case err != nil:
		m.setStatus(fmt.Sprintf("Error: %v", err))
	case count == 0 && zeroMsg != "":
		m.setStatus(zeroMsg)
	case successMsg != nil:
		m.setStatus(successMsg(count))
	}
	return m, nil
}

func (m *PriorityViewModel) priorityStatusChangeAction(status string) keymap.Action {
	s := models.TaskStatus(status)
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			return m.runBulkAction(StatusVerb(s)+" Task", "No tasks were changed", func(count int) string {
				return fmt.Sprintf("Changed %d tasks to %s", count, s)
			}, func(proj *models.Project, taskID string) bool {
				t := proj.GetTaskByID(taskID)
				if t == nil || t.CanTransitionTo(s) != "" {
					return false
				}
				proj.MutateTask(taskID, func(t *models.Task) { t.ApplyTransition(s) })
				return true
			})
		}
		return taskOpsChangeStatus(m, s)
	}
}

func (m *PriorityViewModel) priorityPriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			priority, _ := PriorityForPrefixKey(key)
			return m.runBulkAction("Set Priority", "No tasks were changed", func(count int) string {
				return fmt.Sprintf("Set priority %d on %d tasks", priority, count)
			}, func(proj *models.Project, taskID string) bool {
				t := proj.GetTaskByID(taskID)
				if t == nil || t.Priority == priority {
					return false
				}
				proj.MutateTask(taskID, func(t *models.Task) { t.Priority = priority })
				return true
			})
		}
		return taskOpsChangePriority(m, key)
	}
}

func (m *PriorityViewModel) priorityDueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
			return m.runBulkAction("Set Due Date", "", func(count int) string {
				return fmt.Sprintf("Due date set on %d tasks", count)
			}, func(proj *models.Project, taskID string) bool {
				proj.MutateTask(taskID, func(t *models.Task) {
					d := dueDate
					t.DueDate = &d
					t.HasTime = false
				})
				return true
			})
		}
		return taskOpsSetQuickDueDate(m, days)
	}
}

func (m *PriorityViewModel) priorityDueDateClearAction() (tea.Model, tea.Cmd) {
	if m.hasBulkSelection() {
		return m.runBulkAction("Clear Due Date", "", func(count int) string {
			return fmt.Sprintf("Cleared due date on %d tasks", count)
		}, func(proj *models.Project, taskID string) bool {
			proj.MutateTask(taskID, func(t *models.Task) {
				t.DueDate = nil
				t.HasTime = false
			})
			return true
		})
	}
	return taskOpsClearDueDate(m)
}

func (m *PriorityViewModel) prioritySnoozeDateAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		snoozeDate, _, ok := resolveQuickSnoozeDate(key)
		if !ok {
			return m, nil
		}
		if m.hasBulkSelection() {
			return m.runBulkAction("Snooze", "", func(count int) string {
				return fmt.Sprintf("Snoozed %d tasks", count)
			}, func(proj *models.Project, taskID string) bool {
				proj.MutateTask(taskID, func(t *models.Task) { t.SnoozedUntil = snoozeDate })
				return true
			})
		}
		return taskOpsQuickSnooze(m, snoozeDate)
	}
}

func (m *PriorityViewModel) prioritySectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			return m.runBulkAction("Move to Section", "No tasks were moved", func(count int) string {
				return fmt.Sprintf("Moved %d tasks to %s", count, section)
			}, func(proj *models.Project, taskID string) bool {
				t := proj.GetTaskByID(taskID)
				if t == nil || t.InSection(section) {
					return false
				}
				proj.MutateTask(taskID, func(t *models.Task) { t.SetSection(section) })
				return true
			})
		}
		return taskOpsSectionMove(m, section)
	}
}

func (m *PriorityViewModel) priorityStageChangeAction(stage string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			return m.runBulkAction("Set Stage", "No tasks were changed", func(count int) string {
				return fmt.Sprintf("Set stage on %d tasks", count)
			}, func(proj *models.Project, taskID string) bool {
				t := proj.GetTaskByID(taskID)
				if t == nil || t.Stage == stage {
					return false
				}
				proj.MutateTask(taskID, func(t *models.Task) { t.Stage = stage })
				return true
			})
		}
		return taskOpsChangeStage(m, stage)
	}
}

func (m *PriorityViewModel) priorityFilterEntries(stages []string) []keymap.Entry {
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      m.priorityFilterPrefixAction("#"),
		TogglePriority:    m.priorityFilterPrefixAction("p"),
		ToggleHideDimmed:  m.priorityFilterPrefixAction("h"),
		ToggleGitHubIssue: m.priorityFilterPrefixAction("i"),
		ClearFilters:      m.priorityFilterPrefixAction("x"),
		TypeKey:           m.priorityFilterKeyAction("Ft"),
		StatusKey:         m.priorityFilterKeyAction("Fs"),
		DueDateKey:        m.priorityFilterKeyAction("Fd"),
		AttrDirectKey:     m.priorityFilterKeyAction("Fa"),
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewPriorityView} }
		},
		ScoreKey:         m.priorityFilterKeyAction("FS"),
		StageFilterKey:   m.priorityFilterKeyAction("F."),
		StageFilterNames: stages,
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
