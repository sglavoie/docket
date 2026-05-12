package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for FocusBoardModel.
func (m *FocusBoardModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasItem := func() bool { return m.getRowAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.skipGroupHeaderForward()
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
				m.skipGroupHeaderBackward()
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

	// --- Goto group: g prefix (gg = top, gp = go to project, gs/gy/gc/gx = group-by) ---
	groupByCat := "Group By"
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return FocusBoardSelectMsg{Task: task, Project: project} }
		},
		Extra: []keymap.Entry{
			keymap.Bind("s", "Group by source", groupByCat, func() (tea.Model, tea.Cmd) {
				m.groupBy = boardGroupBySource
				m.applyFilter()
				m.clampCursor()
				return m, nil
			}),
			keymap.Bind("y", "Group by priority", groupByCat, func() (tea.Model, tea.Cmd) {
				m.groupBy = boardGroupByPriority
				m.applyFilter()
				m.clampCursor()
				return m, nil
			}),
			keymap.Bind("c", "Group by score", groupByCat, func() (tea.Model, tea.Cmd) {
				m.groupBy = boardGroupByScore
				m.applyFilter()
				m.clampCursor()
				return m, nil
			}),
			keymap.Bind("x", "Clear grouping", groupByCat, func() (tea.Model, tea.Cmd) {
				m.groupBy = boardGroupByNone
				m.applyFilter()
				m.clampCursor()
				return m, nil
			}),
		},
	})...)

	// --- f prefix: fp (toggle preview only, NOT full FoldGroup) ---
	entries = append(entries, keymap.Prefix("f", "Preview", "View", []keymap.Entry{
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
				model, cmd := taskOpsChangeStatus(m, s)
				m.setMutationFlash(statusMutationFlashBg(s))
				return model, tea.Batch(cmd, mutationFlashTimeout())
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
				p, _ := PriorityForPrefixKey(key)
				model, cmd := taskOpsChangePriority(m, key)
				m.setMutationFlash(priorityMutationFlashBg(p))
				return model, tea.Batch(cmd, mutationFlashTimeout())
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

	// Override RenameTask in TaskPrefixGroup is handled by the builder using v.getTaskRef();
	// FocusBoard needs inline edit mode instead. We patch it via a separate direct binding.
	// (The builder's RenameTask sends an EditTaskMsg; the focus board overrides via "E" binding below.)

	// --- Filter group: "F" prefix, no gate (always active) ---
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handleFocusBoardFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Ft", key)
			}
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Fs", key)
			}
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Fd", key)
			}
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix(prefix, key)
			}
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewFocusBoard} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("FS", key)
			}
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleFocusBoardFilterSubPrefix("F.", key)
			}
		},
		StageFilterNames: stages,
	})...)

	// --- Sort group: 1-5, 0 ---
	doSort := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			handleSortKey(key, &m.sort, func() {
				m.applyFilter()
				m.clampCursor()
				m.setStatus("Sort reset")
			}, func(field SortField) {
				m.applyFilter()
				m.clampCursor()
				dir := "↑"
				if !m.sort.Ascending {
					dir = "↓"
				}
				m.setStatus(fmt.Sprintf("Sort: %s %s", field, dir))
			})
			return m, nil
		}
	}
	entries = append(entries, keymap.SortGroup(keymap.SortDeps{
		SortByPriority: doSort("1"),
		SortByDueDate:  doSort("2"),
		SortByCreated:  doSort("3"),
		SortByStatus:   doSort("4"),
		SortByScore:    doSort("5"),
		ResetSort:      doSort("0"),
	})...)

	// --- View-specific direct bindings ---

	// Tab navigation
	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
	)

	// Search
	entries = append(entries,
		keymap.Bind("/", "Search", "Filtering", func() (tea.Model, tea.Cmd) {
			m.searchInput.SetValue(m.searchQuery)
			return m, m.ActivateSearch()
		}),
	)

	// Quick-add / select-all
	entries = append(entries,
		keymap.Bind("a", "Quick-add board item", "Actions", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.selectAllVisible()
				return m, nil
			}
			m.addMode = true
			m.addInput.SetValue("")
			m.addInput.Focus()
			return m, textinput.Blink
		}),
	)

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
		keymap.Bind("A", "Deselect all", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		}, keymap.AsHidden()),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Navigate to task / open URL", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleEnter()
		}),
		keymap.Bind("x", "Mark item done", "Actions", func() (tea.Model, tea.Cmd) {
			return m.markDone()
		}),
		keymap.Bind("X", "Dismiss item", "Actions", func() (tea.Model, tea.Cmd) {
			return m.dismiss()
		}),
		keymap.Bind("p", "Set priority", "Actions", func() (tea.Model, tea.Cmd) {
			row := m.getRowAtCursor()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowPriorityPickerMsg{Task: row.Task, Project: row.Project} }
		}),
		keymap.Bind("E", "Quick-edit title", "Actions", func() (tea.Model, tea.Cmd) {
			row := m.getRowAtCursor()
			if row == nil || !row.isTask() {
				return m, nil
			}
			m.editMode = true
			m.editIdx = m.cursor
			m.editInput.SetValue(row.Task.Title)
			m.editInput.Focus()
			return m, textinput.Blink
		}),
	)

	// Daily plan reorder
	entries = append(entries,
		keymap.Bind("J", "Move item down in today plan", "Daily Plan", func() (tea.Model, tea.Cmd) {
			return m.handleDailyPlanReorder(1)
		}),
		keymap.Bind("K", "Move item up in today plan", "Daily Plan", func() (tea.Model, tea.Cmd) {
			return m.handleDailyPlanReorder(-1)
		}),
	)

	// Focus mode cycle: List → Tunnel → Zen → List
	entries = append(entries,
		keymap.Bind("ctrl+f", "Cycle focus mode (List/Tunnel/Zen)", "View", func() (tea.Model, tea.Cmd) {
			m.focusMode = (m.focusMode + 1) % 3
			return m, nil
		}),
	)

	// Refresh
	entries = append(entries,
		keymap.Bind("r", "Refresh", "View", func() (tea.Model, tea.Cmd) {
			m.loadItems()
			m.clampCursor()
			m.setStatus("Refreshed")
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

	// Assign to project (hidden, duplicates ctrl+b)
	entries = append(entries,
		keymap.Bind("P", "Assign to project", "Actions", func() (tea.Model, tea.Cmd) {
			row := m.getRowAtCursor()
			if row == nil || row.Project == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: row.Task, SourceProject: row.Project}
			}
		}, keymap.AsHidden()),
	)

	// ctrl+t: switch to Backlog / Plan mode
	entries = append(entries,
		keymap.Bind("ctrl+t", "Switch to Backlog (Plan mode)", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusViewMsg{} }
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Today",
		Entries:  entries,
	}
}
