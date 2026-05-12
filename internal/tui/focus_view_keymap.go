package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for FocusViewModel.
func (m *FocusViewModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasItem := func() bool { return m.getTaskAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
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
	})...)

	// --- Goto group: g prefix (gg = top, gp = go to project) ---
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

	// --- z prefix: zp (toggle preview only) ---
	entries = append(entries, keymap.Prefix("z", "Preview", "View", []keymap.Entry{
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		}),
	}))

	// --- Standard task action groups (Status, Priority, DueDate, Snooze, Move,
	//     LinkedTask, TaskPrefix, Stage, Blocker, CommonTaskAction) ---
	bulkSetDueDate := func(days int) keymap.Action {
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
			return taskOpsSetQuickDueDate(m, days)
		}
	}
	bulkClearDueDate := func() (tea.Model, tea.Cmd) {
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
		return taskOpsClearDueDate(m)
	}
	snoozeDateAction := func(key string) keymap.Action {
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
			return taskOpsQuickSnooze(m, snoozeDate)
		}
	}
	entries = append(entries, buildStandardTaskActionEntries(TaskKeymapDeps{
		View:    m,
		HasItem: hasItem,
		Stages:  stages,
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task != nil {
				m.showDeleteConfirm(task.ShortID)
			}
			return m, nil
		},
		StatusChange: func(status string) keymap.Action {
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
				return taskOpsChangeStatus(m, s)
			}
		},
		PriorityChange: func(key string) keymap.Action {
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
				return taskOpsChangePriority(m, key)
			}
		},
		DueDateSet:   bulkSetDueDate,
		DueDateClear: bulkClearDueDate,
		SnoozeDate:   snoozeDateAction,
		SectionMove: func(section string) keymap.Action {
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
				return taskOpsSectionMove(m, section)
			}
		},
		StageChange: func(stage string) keymap.Action {
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
				return taskOpsChangeStage(m, stage)
			}
		},
		// MarkDone on Suggested tab toggles dismissed instead of marking done.
		MarkDone: func() (tea.Model, tea.Cmd) {
			if m.tabs.Active == focusTabSuggested {
				m.showDismissed = !m.showDismissed
				m.recomputeSuggested()
				m.clampCursor()
				return m, nil
			}
			return taskOpsChangeStatus(m, models.StatusDone)
		},
		OpenURL: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil || task.URL == "" {
				return m, nil
			}
			return m, openBrowserCmd(task.URL)
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		QuickNote: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
		},
	})...)

	// --- Filter group: "F" prefix, no gate ---
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handleFocusFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("Ft", key)
			}
		},
		StatusKey: func(key string) keymap.Action {
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
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusFilterSubPrefix("F.", key)
			}
		},
		StageFilterNames: stages,
	})...)

	// --- View-specific direct bindings ---

	// Enter: navigate to task
	entries = append(entries,
		keymap.Bind("enter", "Navigate to task in project", "Navigation", func() (tea.Model, tea.Cmd) {
			ft := m.getTaskAtCursor()
			if ft == nil {
				return m, nil
			}
			return m, func() tea.Msg { return FocusViewSelectMsg{Task: ft.Task, Project: ft.Project} }
		}),
	)

	// Tab navigation
	entries = append(entries,
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
	)

	// Sub-tab row focus
	entries = append(entries,
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
	)

	// Queue reorder
	entries = append(entries,
		keymap.Bind("J", "Move item down in queue", "Queue", func() (tea.Model, tea.Cmd) {
			return m.handleQueueReorder(1)
		}),
		keymap.Bind("K", "Move item up in queue", "Queue", func() (tea.Model, tea.Cmd) {
			return m.handleQueueReorder(-1)
		}),
	)

	// Dismiss / remove
	entries = append(entries,
		keymap.Bind("x", "Dismiss or remove from queue", "Actions", func() (tea.Model, tea.Cmd) {
			switch m.tabs.Active {
			case focusTabQueue:
				m.removeFromQueue()
			case focusTabSuggested:
				m.toggleDismiss()
			}
			return m, nil
		}),
	)

	// Refresh
	entries = append(entries,
		keymap.Bind("r", "Refresh", "View", func() (tea.Model, tea.Cmd) {
			m.refreshViewState()
			return m, nil
		}),
	)

	// Parent navigation
	entries = append(entries,
		keymap.Bind("n", "Next parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToNext(m.parentRowIndices())
		}),
		keymap.Bind("N", "Previous parent task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToPrev(m.parentRowIndices())
		}),
	)

	// Priority keys: 1-5
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return taskOpsChangePriority(m, key)
			}, keymap.AsHidden()),
		)
	}

	// Multi-select
	entries = append(entries,
		keymap.Bind("v", "Toggle multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection", "Multi-select", func() (tea.Model, tea.Cmd) {
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

	// ctrl+t: switch to Today / Work mode
	entries = append(entries,
		keymap.Bind("ctrl+t", "Switch to Today (Work mode)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusBoardMsg{} }
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Backlog",
		Entries:  entries,
	}
}
