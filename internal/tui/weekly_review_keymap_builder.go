package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildWeeklyReviewKeyMap constructs the declarative KeyMap for WeeklyReviewModel.
func buildWeeklyReviewKeyMap(m *WeeklyReviewModel) *keymap.KeyMap {
	var entries []keymap.Entry
	entries = append(entries, weeklyReviewNavigationEntries(m)...)
	entries = append(entries, weeklyReviewGotoEntries(m)...)
	entries = append(entries, weeklyReviewPreviewEntries(m)...)
	entries = append(entries, buildStandardTaskActionEntries(weeklyReviewTaskActionDeps(m))...)
	entries = append(entries, weeklyReviewFilterEntries(m)...)
	entries = append(entries, weeklyReviewViewSpecificEntries(m)...)

	return &keymap.KeyMap{
		ViewName: "Weekly Review",
		Entries:  entries,
	}
}

func weeklyReviewNavigationEntries(m *WeeklyReviewModel) []keymap.Entry {
	return keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown:   m.weeklyReviewCursorDownAction,
		CursorUp:     m.weeklyReviewCursorUpAction,
		JumpToBottom: m.weeklyReviewJumpToBottomAction,
		PageUp:       m.weeklyReviewPageUpAction,
		PageDown:     m.weeklyReviewPageDownAction,
	})
}

func (m *WeeklyReviewModel) weeklyReviewCursorDownAction() (tea.Model, tea.Cmd) {
	if m.cursor < m.totalItems-1 {
		m.cursor++
	}
	return m, nil
}

func (m *WeeklyReviewModel) weeklyReviewCursorUpAction() (tea.Model, tea.Cmd) {
	if m.cursor > 0 {
		m.cursor--
	}
	return m, nil
}

func (m *WeeklyReviewModel) weeklyReviewJumpToBottomAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	if m.totalItems > 0 {
		m.cursor = m.totalItems - 1
	}
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *WeeklyReviewModel) weeklyReviewPageUpAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalItems, -HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func (m *WeeklyReviewModel) weeklyReviewPageDownAction() (tea.Model, tea.Cmd) {
	old := m.cursor
	jumpCursor(&m.cursor, m.totalItems, HalfPage(m.Height()))
	return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
}

func weeklyReviewGotoEntries(m *WeeklyReviewModel) []keymap.Entry {
	return keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: m.weeklyReviewGoToProjectAction,
		Extra: []keymap.Entry{
			keymap.Bind("P", "Focus Stale Projects section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionStaleProjects)),
			keymap.Bind("i", "Focus Inbox Backlog section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionInboxBacklog)),
			keymap.Bind("b", "Focus Blocked Work section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionBlockedWork)),
			keymap.Bind("o", "Focus Overcommitted section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionOvercommitted)),
			keymap.Bind("t", "Focus Stale Tasks section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionStaleTasks)),
			keymap.Bind("m", "Focus Priority Mismatch section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionPriorityMismatch)),
			keymap.Bind("w", "Focus Upcoming Week section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionUpcomingWeek)),
			keymap.Bind("r", "Focus Recurring Overview section", "Sections", m.weeklyReviewFocusSectionAction(WeeklySectionRecurringOverview)),
		},
	})
}

func (m *WeeklyReviewModel) weeklyReviewFocusSectionAction(section WeeklyReviewSection) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		return m.toggleSectionFocus(section)
	}
}

func (m *WeeklyReviewModel) weeklyReviewGoToProjectAction() (tea.Model, tea.Cmd) {
	if wp := m.getProjectAtCursor(); wp != nil {
		return m, func() tea.Msg { return WeeklyReviewSelectProjectMsg{Project: wp.Project} }
	}
	task, project := m.getTaskRef()
	if task == nil {
		return m, nil
	}
	return m, func() tea.Msg { return WeeklyReviewSelectTaskMsg{Task: task, Project: project} }
}

func weeklyReviewPreviewEntries(m *WeeklyReviewModel) []keymap.Entry {
	return []keymap.Entry{
		keymap.Prefix("f", "View", "View", []keymap.Entry{
			keymap.Bind("a", "Toggle section fold", "Folds", m.weeklyReviewToggleSectionAction),
			keymap.Bind("p", "Toggle inline preview", "View", m.weeklyReviewTogglePreviewAction),
		}),
	}
}

func (m *WeeklyReviewModel) weeklyReviewToggleSectionAction() (tea.Model, tea.Cmd) {
	return m.toggleCurrentSection()
}

func (m *WeeklyReviewModel) weeklyReviewTogglePreviewAction() (tea.Model, tea.Cmd) {
	m.previewMode = !m.previewMode
	return m, nil
}

func weeklyReviewTaskActionDeps(m *WeeklyReviewModel) TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.weeklyReviewHasItem,
		ToggleHelp:       m.weeklyReviewToggleHelpAction,
		SelectedTaskRefs: m.weeklyReviewSelectedTaskRefs,
		DeleteTask:       m.weeklyReviewDeleteTaskAction,
		OpenURL:          m.weeklyReviewOpenURLAction,
		MoveToProject:    m.weeklyReviewMoveToProjectAction,
		QuickNote:        m.weeklyReviewQuickNoteAction,
		StateChange:      m.weeklyReviewStatusChangeAction,
		PriorityChange:   m.weeklyReviewPriorityChangeAction,
		DueDateSet:       m.weeklyReviewDueDateSetAction,
		DueDateClear:     m.weeklyReviewDueDateClearAction,
		SnoozeDate:       m.weeklyReviewSnoozeDateAction,
		SectionMove:      m.weeklyReviewSectionMoveAction,
		MarkDone:         m.weeklyReviewMarkDoneAction,
	}
}

func (m *WeeklyReviewModel) weeklyReviewHasItem() bool {
	return m.getTaskAtCursor() != nil
}

func (m *WeeklyReviewModel) weeklyReviewSelectedTaskRefs() []TaskProjectRef {
	if !m.hasBulkSelection() {
		return nil
	}
	return m.getSelectedBulkItems()
}

func (m *WeeklyReviewModel) weeklyReviewToggleHelpAction() (tea.Model, tea.Cmd) {
	return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
}

func (m *WeeklyReviewModel) weeklyReviewDeleteTaskAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task != nil {
		m.showDeleteConfirm(task.ShortID)
	}
	return m, nil
}

func (m *WeeklyReviewModel) weeklyReviewOpenURLAction() (tea.Model, tea.Cmd) {
	task, _ := m.getTaskRef()
	if task == nil || task.URL == "" {
		return m, nil
	}
	return m, openBrowserCmd(task.URL)
}

func (m *WeeklyReviewModel) weeklyReviewMoveToProjectAction() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
}

func (m *WeeklyReviewModel) weeklyReviewQuickNoteAction() (tea.Model, tea.Cmd) {
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}
	return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
}

func (m *WeeklyReviewModel) hasBulkSelection() bool {
	return m.multiSelectMode && m.SelectedCount() > 0
}

func (m *WeeklyReviewModel) runTrackedAction(category string, action func() (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	model, cmd := action()
	if m.err == nil {
		m.trackMutation(category)
	}
	return model, cmd
}

func (m *WeeklyReviewModel) weeklyReviewStatusChangeAction(state string) keymap.Action {
	s := models.WorkflowState(state)
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			if cmd := bulkListStateCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), s); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m.runTrackedAction("status", func() (tea.Model, tea.Cmd) { return m, wfChangeState(m, s) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewPriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			priority, _ := PriorityForPrefixKey(key)
			if cmd := bulkListPriorityCmd(m.bulk, m.getSelectedBulkItems(), priority); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m.runTrackedAction("priority", func() (tea.Model, tea.Cmd) { return m, wfChangePriority(m, key) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewDueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			if cmd := bulkListDueDateCmd(m.bulk, m.getSelectedBulkItems(), days); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		return m.runTrackedAction("due_date", func() (tea.Model, tea.Cmd) { return m, wfSetQuickDueDate(m, days) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewDueDateClearAction() (tea.Model, tea.Cmd) {
	if m.hasBulkSelection() {
		if cmd := bulkListClearDueDateCmd(m.bulk, m.getSelectedBulkItems()); cmd != nil {
			return m, cmd
		}
		return m, nil
	}
	return m.runTrackedAction("due_date", func() (tea.Model, tea.Cmd) { return m, wfClearDueDate(m) })
}

func (m *WeeklyReviewModel) weeklyReviewSnoozeDateAction(key string) keymap.Action {
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
		return m.runTrackedAction("snooze", func() (tea.Model, tea.Cmd) { return m, wfQuickSnooze(m, snoozeDate) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewSectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.hasBulkSelection() {
			cmd, ok := bulkListSectionMoveCmd(m.bulk, m.enabledWorkflowRules(), m.getSelectedBulkItems(), section)
			if !ok {
				m.setStatus(fmt.Sprintf("Unknown section %q", section))
			}
			return m, cmd
		}
		return m.runTrackedAction("move", func() (tea.Model, tea.Cmd) { return m, wfSectionMove(m, section) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewMarkDoneAction() (tea.Model, tea.Cmd) {
	return m.runTrackedAction("status", func() (tea.Model, tea.Cmd) { return m, wfChangeState(m, models.StateDone) })
}

func weeklyReviewFilterEntries(m *WeeklyReviewModel) []keymap.Entry {
	return keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      m.weeklyReviewFilterPrefixAction("#"),
		TogglePriority:    m.weeklyReviewFilterPrefixAction("p"),
		ToggleHideDimmed:  m.weeklyReviewFilterPrefixAction("h"),
		ToggleGitHubIssue: m.weeklyReviewFilterPrefixAction("i"),
		ToggleParked:      m.weeklyReviewFilterPrefixAction("k"),
		ClearFilters:      m.weeklyReviewFilterPrefixAction("x"),
		TypeKey:           m.weeklyReviewFilterKeyAction("Ft"),
		StateKey:          m.weeklyReviewFilterKeyAction("Fs"),
		DueDateKey:        m.weeklyReviewFilterKeyAction("Fd"),
		AttrDirectKey:     m.weeklyReviewFilterKeyAction("Fa"),
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleWeeklyReviewFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewWeeklyReview} }
		},
		ScoreKey: m.weeklyReviewFilterKeyAction("FS"),
	})
}

func (m *WeeklyReviewModel) weeklyReviewFilterPrefixAction(prefix string) keymap.Action {
	return func() (tea.Model, tea.Cmd) { return m.handleWeeklyReviewFilterPrefix(prefix) }
}

func (m *WeeklyReviewModel) weeklyReviewFilterKeyAction(prefix string) func(string) keymap.Action {
	return func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return m.handleWeeklyReviewFilterSubPrefix(prefix, key) }
	}
}
