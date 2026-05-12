package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for AllTasksViewModel.
func (m *AllTasksViewModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasItem := func() bool { return m.getTaskAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < m.totalNavigableItems()-1 {
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
			n := m.totalNavigableItems()
			if n > 0 {
				m.cursor = n - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, m.totalNavigableItems(), -HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, m.totalNavigableItems(), HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)

	// --- Goto group: g prefix (gg = top, gp = go to project) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			item := m.getTaskAtCursor()
			if item == nil {
				return m, nil
			}
			return m, func() tea.Msg { return AllTasksSelectMsg{Task: item.Task, Project: item.Project} }
		},
	})...)

	// --- f prefix: fp, fR, fM ---
	entries = append(entries, keymap.Prefix("f", "View", "View", []keymap.Entry{
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		}),
		keymap.Bind("R", "Expand all groups", "View", func() (tea.Model, tea.Cmd) {
			ExpandAllSections(m.collapsedGroups)
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("M", "Collapse all groups", "View", func() (tea.Model, tea.Cmd) {
			groupNames := make([]string, len(m.groups))
			for i, g := range m.groups {
				groupNames[i] = g.Name
			}
			CollapseAllSections(m.collapsedGroups, groupNames)
			m.calculateNavigation()
			if len(m.groupStarts) > 0 {
				best := m.groupStarts[0]
				for _, start := range m.groupStarts {
					if start <= m.cursor {
						best = start
					}
				}
				m.cursor = best
			}
			m.clampCursor()
			return m, nil
		}),
	}))

	// --- Task action groups (Status, Priority, DueDate, Snooze, Move, Linked, TaskPrefix, Stage, Blocker, CommonTaskAction) ---
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
					return m.handleBulkStatusChange(s)
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
					return m.handleBulkPriorityChange(p)
				}
				p, _ := PriorityForPrefixKey(key)
				model, cmd := taskOpsChangePriority(m, key)
				m.setMutationFlash(priorityMutationFlashBg(p))
				return model, tea.Batch(cmd, mutationFlashTimeout())
			}
		},
		DueDateSet: func(days int) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					return m.handleBulkDueDateChange(days)
				}
				return taskOpsSetQuickDueDate(m, days)
			}
		},
		DueDateClear: func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				return m.handleBulkClearDueDate()
			}
			return taskOpsClearDueDate(m)
		},
		SectionMove: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					return m.handleBulkSectionMove(section)
				}
				return taskOpsSectionMove(m, section)
			}
		},
		MarkDone: func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				return m.handleBulkStatusChange(models.StatusDone)
			}
			model, cmd := taskOpsChangeStatus(m, models.StatusDone)
			m.setMutationFlash(statusMutationFlashBg(models.StatusDone))
			return model, tea.Batch(cmd, mutationFlashTimeout())
		},
	})...)

	// --- Filter group: "F" prefix, no gate ---
	// Build base filter group, then add AllTasks-specific entries.
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleAllTasksFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleAllTasksFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleAllTasksFilterPrefix("h") },
		ClearFilters: func() (tea.Model, tea.Cmd) {
			clearCommonFilters(&m.filter)
			m.projectFilter = make(map[string]bool)
			m.applyFiltersAndGroup()
			m.clampCursor()
			return m, nil
		},
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleAllTasksFilterSubPrefix("Ft", key)
			}
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleAllTasksFilterSubPrefix("Fs", key)
			}
		},
		DueDateKey: nil, // AllTasks uses a simple cycle "d" key, NOT sub-prefix
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleAllTasksFilterSubPrefix("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleAllTasksFilterSubPrefix(prefix, key)
			}
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewAllTasks} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleAllTasksFilterSubPrefix("FS", key)
			}
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleAllTasksFilterSubPrefix("F.", key)
			}
		},
		StageFilterNames: stages,
	})
	// Inject AllTasks-specific children into the F prefix node.
	if len(filterEntries) > 0 && filterEntries[0].PrefixNode != nil {
		filterEntries[0].PrefixNode.Children = append(filterEntries[0].PrefixNode.Children,
			keymap.Bind("d", "Cycle due date filter", "Filters", func() (tea.Model, tea.Cmd) {
				m.filter.DueDate = (m.filter.DueDate + 1) % (DueDateNone + 1)
				m.applyFiltersAndGroup()
				m.clampCursor()
				return m, nil
			}),
			keymap.Bind("P", "Toggle project filter", "Filters", func() (tea.Model, tea.Cmd) {
				m.toggleProjectFilter()
				return m, nil
			}),
		)
	}
	entries = append(entries, filterEntries...)

	// --- Sort group: 1-5 for sort fields, 0 to reset ---
	doSort := func(key string) keymap.Action {
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
	entries = append(entries, keymap.SortGroup(keymap.SortDeps{
		SortByPriority: doSort("1"),
		SortByDueDate:  doSort("2"),
		SortByCreated:  doSort("3"),
		SortByStatus:   doSort("4"),
		SortByScore:    doSort("5"),
		ResetSort:      doSort("0"),
	})...)

	// --- View-specific direct bindings ---

	// Enter: show task detail
	entries = append(entries,
		keymap.Bind("enter", "View task details", "Actions", func() (tea.Model, tea.Cmd) {
			if item := m.getTaskAtCursor(); item != nil {
				return m, func() tea.Msg {
					return ShowTaskDetailMsg{Task: item.Task, Project: item.Project}
				}
			}
			return m, nil
		}),
	)

	// Group navigation
	entries = append(entries,
		keymap.Bind("n", "Next group header", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.goToNextGroup()
		}),
		keymap.Bind("N", "Previous group header", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.goToPrevGroup()
		}),
	)

	// Cycle grouping
	entries = append(entries,
		keymap.Bind("ctrl+g", "Cycle grouping mode", "View", func() (tea.Model, tea.Cmd) {
			m.cycleGroupBy()
			return m, nil
		}),
	)

	// Group collapse/expand
	entries = append(entries,
		keymap.Bind("tab", "Collapse/expand group", "View", func() (tea.Model, tea.Cmd) {
			m.toggleGroupCollapse()
			return m, nil
		}),
	)

	// Tab navigation — cycle through all tabs (including overflow), wrapping.
	// Entering the overflow zone opens the picker automatically.
	cycleTab := func(delta int) {
		total := len(m.allTabEntries)
		if total <= 1 {
			return
		}
		m.activeTabIdx = ((m.activeTabIdx+delta)%total + total) % total
		m.tabs.Active = min(m.activeTabIdx, len(m.tabEntries)-1)
		if m.activeTabIdx >= overflowTabCap-1 && len(m.allTabEntries) > overflowTabCap {
			// Clamp cursor to the entry we just landed on.
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
	entries = append(entries,
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			cycleTab(-1)
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			cycleTab(1)
			return m, nil
		}, keymap.AsHidden()),
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

	// Smart lists and tab management
	entries = append(entries,
		keymap.Bind("+", "Create smart list tab", "Smart Lists", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFilterBuilderMsg{} }
		}),
		keymap.Bind("E", "Edit current smart list tab", "Smart Lists", func() (tea.Model, tea.Cmd) {
			entry := m.activeTabEntry()
			if entry != nil && !entry.IsBuiltIn && !entry.IsOverflow {
				sf := m.config.GetSavedFilter(entry.SavedFilterID)
				if sf != nil && sf.Mode == "criteria" {
					filter := *sf
					return m, func() tea.Msg {
						return ShowFilterEditorMsg{Filter: &filter}
					}
				}
			}
			return m, nil
		}),
		keymap.Bind("X", "Delete current smart list tab", "Smart Lists", func() (tea.Model, tea.Cmd) {
			entry := m.activeTabEntry()
			if entry != nil && !entry.IsBuiltIn && !entry.IsOverflow {
				m.confirm.Show("delete-smart-list", fmt.Sprintf("Delete smart list %q?", entry.Name))
			}
			return m, nil
		}),
		keymap.Bind("{", "Move tab left", "Actions", func() (tea.Model, tea.Cmd) {
			m.moveTabLeft()
			return m, nil
		}),
		keymap.Bind("}", "Move tab right", "Actions", func() (tea.Model, tea.Cmd) {
			m.moveTabRight()
			return m, nil
		}),
	)

	// Export
	entries = append(entries,
		keymap.Bind("ctrl+e", "Export task's project to Markdown", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ExportProjectMsg{} }
		}),
	)

	return &keymap.KeyMap{
		ViewName: "All Tasks",
		Entries:  entries,
	}
}
