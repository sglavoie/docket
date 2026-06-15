package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for DailyReviewModel.
func (m *DailyReviewModel) buildKeyMap() *keymap.KeyMap {
	stages := m.dailyReviewStages()

	var entries []keymap.Entry
	entries = append(entries, m.dailyReviewNavigationEntries()...)
	entries = append(entries, m.dailyReviewTaskActionEntries(stages)...)
	entries = append(entries, m.dailyReviewFilterEntries(stages)...)
	entries = append(entries, m.dailyReviewDirectEntries()...)

	return &keymap.KeyMap{
		ViewName: "Daily Review",
		Entries:  entries,
	}
}

func (m *DailyReviewModel) dailyReviewStages() []string {
	if m.config == nil {
		return nil
	}
	return m.config.Stages
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

func (m *DailyReviewModel) dailyReviewTaskActionEntries(stages []string) []keymap.Entry {
	return buildStandardTaskActionEntries(m.dailyReviewTaskActionDeps(stages))
}

func (m *DailyReviewModel) dailyReviewTaskActionDeps(stages []string) TaskKeymapDeps {
	return TaskKeymapDeps{
		View:             m,
		HasItem:          m.dailyReviewHasItem,
		Stages:           stages,
		ToggleHelp:       m.dailyReviewToggleHelp,
		SelectedTaskRefs: m.dailyReviewSelectedTaskRefs,
		DeleteTask:       m.dailyReviewDeleteTask,
		StatusChange:     m.dailyReviewStatusChangeAction,
		PriorityChange:   m.dailyReviewPriorityChangeAction,
		DueDateSet:       m.dailyReviewDueDateSetAction,
		DueDateClear:     m.dailyReviewClearDueDate,
		SnoozeDate:       m.dailyReviewSnoozeDateAction,
		SectionMove:      m.dailyReviewSectionMoveAction,
		StageChange:      m.dailyReviewStageChangeAction,
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

func (m *DailyReviewModel) dailyReviewStatusChangeAction(status string) keymap.Action {
	s := models.TaskStatus(status)
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, StatusVerb(s)+" Task",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.CanTransitionTo(s) != "" {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.ApplyTransition(s) })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were changed")
			} else {
				m.setStatus(fmt.Sprintf("Changed %d tasks to %s", count, s))
			}
			return m, nil
		}
		return reviewChangeStatus(m, s)
	}
}

func (m *DailyReviewModel) dailyReviewPriorityChangeAction(key string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			p, _ := PriorityForPrefixKey(key)
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Set Priority",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.Priority == p {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.Priority = p })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were changed")
			} else {
				m.setStatus(fmt.Sprintf("Set priority %d on %d tasks", p, count))
			}
			return m, nil
		}
		return reviewChangePriority(m, key)
	}
}

func (m *DailyReviewModel) dailyReviewDueDateSetAction(days int) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Set Due Date",
				func(proj *models.Project, taskID string) bool {
					proj.MutateTask(taskID, func(t *models.Task) {
						d := dueDate
						t.DueDate = &d
						t.HasTime = false
					})
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Due date set on %d tasks", count))
			}
			return m, nil
		}
		return reviewSetQuickDueDate(m, days)
	}
}

func (m *DailyReviewModel) dailyReviewClearDueDate() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && m.SelectedCount() > 0 {
		items := m.getSelectedBulkItems()
		count, err := taskOpsApplyBulk(m, items, "Clear Due Date",
			func(proj *models.Project, taskID string) bool {
				proj.MutateTask(taskID, func(t *models.Task) {
					t.DueDate = nil
					t.HasTime = false
				})
				return true
			})
		m.ExitMultiSelect()
		if err != nil {
			m.setStatus(fmt.Sprintf("Error: %v", err))
		} else {
			m.setStatus(fmt.Sprintf("Cleared due date on %d tasks", count))
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
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Snooze",
				func(proj *models.Project, taskID string) bool {
					proj.MutateTask(taskID, func(t *models.Task) { t.SnoozedUntil = snoozeDate })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Snoozed %d tasks", count))
			}
			return m, nil
		}
		return reviewQuickSnooze(m, snoozeDate)
	}
}

func (m *DailyReviewModel) dailyReviewSectionMoveAction(section string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Move to Section",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.InSection(section) {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.SetSection(section) })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were moved")
			} else {
				m.setStatus(fmt.Sprintf("Moved %d tasks to %s", count, section))
			}
			return m, nil
		}
		return reviewSectionMove(m, section)
	}
}

func (m *DailyReviewModel) dailyReviewStageChangeAction(stage string) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Set Stage",
				func(proj *models.Project, taskID string) bool {
					t := proj.GetTaskByID(taskID)
					if t == nil || t.Stage == stage {
						return false
					}
					proj.MutateTask(taskID, func(t *models.Task) { t.Stage = stage })
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else if count == 0 {
				m.setStatus("No tasks were changed")
			} else {
				m.setStatus(fmt.Sprintf("Set stage on %d tasks", count))
			}
			return m, nil
		}
		return reviewChangeStage(m, stage)
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

func (m *DailyReviewModel) dailyReviewFilterEntries(stages []string) []keymap.Entry {
	return keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Ft", key) }
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Fd", key) }
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("Fa", key) }
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewDailyReview} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleDailyReviewFilterSubPrefix("F.", key) }
		},
		StageFilterNames: stages,
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
		keymap.Bind("space", "Toggle selection (multi-select) / hold to peek", "Multi-select", func() (tea.Model, tea.Cmd) {
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
