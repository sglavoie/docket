package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildWeeklyReviewKeyMap constructs the declarative KeyMap for WeeklyReviewModel.
func buildWeeklyReviewKeyMap(m *WeeklyReviewModel) *keymap.KeyMap {
	stages := weeklyReviewStages(m)
	var entries []keymap.Entry
	entries = append(entries, weeklyReviewNavigationEntries(m)...)
	entries = append(entries, weeklyReviewGotoEntries(m)...)
	entries = append(entries, weeklyReviewPreviewEntries(m)...)
	entries = append(entries, buildStandardTaskActionEntries(weeklyReviewTaskActionDeps(m, stages))...)
	entries = append(entries, weeklyReviewFilterEntries(m, stages)...)
	entries = append(entries, weeklyReviewViewSpecificEntries(m)...)

	return &keymap.KeyMap{
		ViewName: "Weekly Review",
		Entries:  entries,
	}
}

func weeklyReviewStages(m *WeeklyReviewModel) []string {
	if m.config == nil {
		return nil
	}
	return m.config.Stages
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

func weeklyReviewTaskActionDeps(m *WeeklyReviewModel, stages []string) TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.weeklyReviewHasItem,
		Stages:           stages,
		ToggleHelp:       m.weeklyReviewToggleHelpAction,
		SelectedTaskRefs: m.weeklyReviewSelectedTaskRefs,
		DeleteTask:       m.weeklyReviewDeleteTaskAction,
		OpenURL:          m.weeklyReviewOpenURLAction,
		MoveToProject:    m.weeklyReviewMoveToProjectAction,
		QuickNote:        m.weeklyReviewQuickNoteAction,
		StatusChange:     m.weeklyReviewStatusChangeAction,
		PriorityChange:   m.weeklyReviewPriorityChangeAction,
		DueDateSet:       m.weeklyReviewDueDateSetAction,
		DueDateClear:     m.weeklyReviewDueDateClearAction,
		SnoozeDate:       m.weeklyReviewSnoozeDateAction,
		SectionMove:      m.weeklyReviewSectionMoveAction,
		StageChange:      m.weeklyReviewStageChangeAction,
		MarkDone:         m.weeklyReviewMarkDoneAction,
		ToggleActive:     m.weeklyReviewToggleActiveAction,
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

func (m *WeeklyReviewModel) runBulkAction(
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

func (m *WeeklyReviewModel) runTrackedAction(category string, action func() (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	model, cmd := action()
	if m.err == nil {
		m.trackMutation(category)
	}
	return model, cmd
}

func (m *WeeklyReviewModel) weeklyReviewStatusChangeAction(status string) keymap.Action {
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
		return m.runTrackedAction("status", func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(m, s) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewPriorityChangeAction(key string) keymap.Action {
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
		return m.runTrackedAction("priority", func() (tea.Model, tea.Cmd) { return taskOpsChangePriority(m, key) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewDueDateSetAction(days int) keymap.Action {
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
		return m.runTrackedAction("due_date", func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(m, days) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewDueDateClearAction() (tea.Model, tea.Cmd) {
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
	return m.runTrackedAction("due_date", func() (tea.Model, tea.Cmd) { return taskOpsClearDueDate(m) })
}

func (m *WeeklyReviewModel) weeklyReviewSnoozeDateAction(key string) keymap.Action {
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
		return m.runTrackedAction("snooze", func() (tea.Model, tea.Cmd) { return taskOpsQuickSnooze(m, snoozeDate) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewSectionMoveAction(section string) keymap.Action {
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
		return m.runTrackedAction("move", func() (tea.Model, tea.Cmd) { return taskOpsSectionMove(m, section) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewStageChangeAction(stage string) keymap.Action {
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
		return m.runTrackedAction("stage", func() (tea.Model, tea.Cmd) { return taskOpsChangeStage(m, stage) })
	}
}

func (m *WeeklyReviewModel) weeklyReviewMarkDoneAction() (tea.Model, tea.Cmd) {
	return m.runTrackedAction("status", func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(m, models.StatusDone) })
}

func (m *WeeklyReviewModel) weeklyReviewToggleActiveAction() (tea.Model, tea.Cmd) {
	return m.runTrackedAction("active", func() (tea.Model, tea.Cmd) { return taskOpsToggleActive(m) })
}

func weeklyReviewFilterEntries(m *WeeklyReviewModel, stages []string) []keymap.Entry {
	return keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      m.weeklyReviewFilterPrefixAction("#"),
		TogglePriority:    m.weeklyReviewFilterPrefixAction("p"),
		ToggleHideDimmed:  m.weeklyReviewFilterPrefixAction("h"),
		ToggleGitHubIssue: m.weeklyReviewFilterPrefixAction("i"),
		ToggleParked:      m.weeklyReviewFilterPrefixAction("k"),
		ClearFilters:      m.weeklyReviewFilterPrefixAction("x"),
		TypeKey:           m.weeklyReviewFilterKeyAction("Ft"),
		StatusKey:         m.weeklyReviewFilterKeyAction("Fs"),
		DueDateKey:        m.weeklyReviewFilterKeyAction("Fd"),
		AttrDirectKey:     m.weeklyReviewFilterKeyAction("Fa"),
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleWeeklyReviewFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewWeeklyReview} }
		},
		ScoreKey:         m.weeklyReviewFilterKeyAction("FS"),
		StageFilterKey:   m.weeklyReviewFilterKeyAction("F."),
		StageFilterNames: stages,
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
