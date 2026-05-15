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
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasItem := func() bool { return m.getTaskAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
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
	})...)

	// --- Goto group: g prefix (gg = top, gp = go to project, gt = go to task, gh = activity) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return PriorityViewSelectMsg{Task: task} }
		},
		Extra: []keymap.Entry{
			keymap.Bind("t", "Go to task in project tree", "Navigation", func() (tea.Model, tea.Cmd) {
				pt := m.getTaskAtCursor()
				if pt == nil {
					return m, nil
				}
				return m, func() tea.Msg { return PriorityViewSelectMsg{Task: pt.Task} }
			}),
			keymap.Bind("h", "Open task activity log", "Navigation", func() (tea.Model, tea.Cmd) {
				pt := m.getTaskAtCursor()
				if pt == nil {
					return m, nil
				}
				project := m.project
				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: pt.Task, Project: project}
				}
			}),
		},
	})...)

	// --- f prefix: fp (toggle preview only) ---
	entries = append(entries, keymap.Prefix("f", "Preview", "View", []keymap.Entry{
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		}),
	}))

	// --- Standard task action groups (Status through CommonTaskAction) ---
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
		DueDateSet: func(days int) keymap.Action {
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
		},
		DueDateClear: func() (tea.Model, tea.Cmd) {
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
		},
		SnoozeDate: func(key string) keymap.Action {
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
		},
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
	})...)

	// --- Filter group: "F" prefix, no gate ---
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix("Ft", key) }
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix("Fd", key) }
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix("Fa", key) }
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewPriorityView} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handlePriorityFilterSubPrefix("F.", key) }
		},
		StageFilterNames: stages,
	})
	// Inject priority-view-specific filter: Fr = toggle actionable
	if len(filterEntries) > 0 && filterEntries[0].PrefixNode != nil {
		filterEntries[0].PrefixNode.Children = append(filterEntries[0].PrefixNode.Children,
			keymap.Bind("r", "Toggle actionable filter", "Filters", func() (tea.Model, tea.Cmd) {
				m.filter.Actionable = !m.filter.Actionable
				m.loadActionableTasks()
				return m, nil
			}),
		)
	}
	entries = append(entries, filterEntries...)

	// --- View-specific direct bindings ---

	// Enter: open task detail
	entries = append(entries,
		keymap.Bind("enter", "View task details", "Actions", func() (tea.Model, tea.Cmd) {
			pt := m.getTaskAtCursor()
			if pt == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowTaskDetailMsg{Task: pt.Task, Project: project} }
		}),
	)

	// Tab: toggle waiting on section
	entries = append(entries,
		keymap.Bind("tab", "Toggle waiting section", "View", func() (tea.Model, tea.Cmd) {
			m.waitingOnCollapsed = !m.waitingOnCollapsed
			return m, nil
		}),
	)

	// Tab navigation (shift+left/right also handle sub-tab row when focused)
	entries = append(entries,
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.subTabRowFocused && len(m.subTabNames) > 1 {
				total := len(m.subTabNames) + 1
				CycleTab(&m.subTab, total, 1)
				m.recalculateNavigation()
				m.clampCursor()
			} else {
				m.tabs.Cycle(1)
				m.subTab = 0
				m.subTabRowFocused = false
				m.recalculateNavigation()
				m.clampCursor()
			}
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.subTabRowFocused && len(m.subTabNames) > 1 {
				total := len(m.subTabNames) + 1
				CycleTab(&m.subTab, total, -1)
				m.recalculateNavigation()
				m.clampCursor()
			} else {
				m.tabs.Cycle(-1)
				m.subTab = 0
				m.subTabRowFocused = false
				m.recalculateNavigation()
				m.clampCursor()
			}
			return m, nil
		}),
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

	// Suggestion keys
	entries = append(entries,
		keymap.Bind("w", "Start suggested task", "Suggestion", func() (tea.Model, tea.Cmd) {
			return m.startSuggestedTask()
		}),
		keymap.Bind("x", "Dismiss current suggestion", "Suggestion", func() (tea.Model, tea.Cmd) {
			m.dismissSuggestion()
			return m, nil
		}),
	)

	// A: open all tasks view (or deselect all in multi-select)
	entries = append(entries,
		keymap.Bind("A", "All tasks", "View", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAll()
				return m, nil
			}
			return m, func() tea.Msg { return ShowAllTasksMsg{} }
		}),
	)

	// Search
	entries = append(entries,
		keymap.Bind("/", "Search tasks", "Filter", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
	)

	// Refresh (clears dismissed suggestions)
	entries = append(entries,
		keymap.Bind("r", "Refresh (clears dismissed)", "View", func() (tea.Model, tea.Cmd) {
			m.dismissedIDs = make(map[string]bool)
			m.loadActionableTasks()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// Priority keys: 1-5 (direct priority change, not sort)
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return m.changeTaskPriority(int(key[0] - '0'))
			}, keymap.AsHidden()),
		)
	}

	// Multi-select
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
	)

	return &keymap.KeyMap{
		ViewName: "Priority",
		Entries:  entries,
	}
}
