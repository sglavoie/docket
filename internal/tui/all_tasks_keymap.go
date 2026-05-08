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

	// --- z prefix: zp, zR, zM ---
	entries = append(entries, keymap.Prefix("z", "View", "View", []keymap.Entry{
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

	// --- Status group: "," prefix, gated on hasItem ---
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus: func(status string) keymap.Action {
			s := models.TaskStatus(status)
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					return m.handleBulkStatusChange(s)
				}
				return taskOpsChangeStatus(m, s)
			}
		},
		HasRecurrence:  nil,
		SkipRecurrence: nil,
		DeferChildren:  nil,
	})
	if len(statusEntries) > 0 && statusEntries[0].PrefixNode != nil {
		statusEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, statusEntries...)

	// --- Priority group: "!" prefix, gated on hasItem ---
	priorityEntries := keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					p, _ := PriorityForPrefixKey(key)
					return m.handleBulkPriorityChange(p)
				}
				return taskOpsChangePriority(m, key)
			}
		},
	})
	if len(priorityEntries) > 0 && priorityEntries[0].PrefixNode != nil {
		priorityEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, priorityEntries...)

	// --- Due date group: "d" prefix, gated on hasItem ---
	bulkSetDueDate := func(days int) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				return m.handleBulkDueDateChange(days)
			}
			return taskOpsSetQuickDueDate(m, days)
		}
	}
	bulkClearDueDate := func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			return m.handleBulkClearDueDate()
		}
		return taskOpsClearDueDate(m)
	}
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:     bulkSetDueDate(0),
		DueTomorrow:  bulkSetDueDate(1),
		DueInAWeek:   bulkSetDueDate(7),
		DueInAMonth:  bulkSetDueDate(30),
		ClearDueDate: bulkClearDueDate,
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetDueDateMsg{Task: task, Project: project} }
		},
	})
	if len(dueDateEntries) > 0 && dueDateEntries[0].PrefixNode != nil {
		dueDateEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, dueDateEntries...)

	// --- Snooze group: "s" prefix, gated on hasItem ---
	snoozeDateAction := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate(key)
			if !ok {
				return m, nil
			}
			return taskOpsQuickSnooze(m, snoozeDate)
		}
	}
	snoozeEntries := keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDateAction("t"),
		SnoozeOneWeek:    snoozeDateAction("w"),
		SnoozeOneMonth:   snoozeDateAction("m"),
		SnoozeNextMonday: snoozeDateAction("n"),
		ClearSnooze:      snoozeDateAction("0"),
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
	})
	if len(snoozeEntries) > 0 && snoozeEntries[0].PrefixNode != nil {
		snoozeEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, snoozeEntries...)

	// --- Move group: "m" prefix, gated on hasItem ---
	bulkMove := func(section string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				return m.handleBulkSectionMove(section)
			}
			return taskOpsSectionMove(m, section)
		}
	}
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: bulkMove,
		MoveToTop:     func() (tea.Model, tea.Cmd) { return m, nil },
		MoveToBottom:  func() (tea.Model, tea.Cmd) { return m, nil },
	})
	if len(moveEntries) > 0 && moveEntries[0].PrefixNode != nil {
		moveEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, moveEntries...)

	// --- Linked task group: "c" prefix, gated on hasItem ---
	linkedEntries := keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
			}
		},
		CreateDependent: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeDependent}
			}
		},
		CreateSubtask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeSubtask}
			}
		},
		CreateFollowUp: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeFollowUp}
			}
		},
		NavigateLinked: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowLinkedTaskSelectorMsg{Task: task, Project: project} }
		},
		CrossProjectLink: nil,
		UnlinkTask:       nil,
	})
	if len(linkedEntries) > 0 && linkedEntries[0].PrefixNode != nil {
		linkedEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, linkedEntries...)

	// --- Task prefix group: "t" prefix, gated on hasItem ---
	taskPrefixEntries := keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: true} }
		},
		NewTaskWithTemplate: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: false} }
		},
		RenameTask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ChangeSection: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowSectionPickerMsg{Task: task, Project: project} }
		},
		OpenEditor: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditTaskContentMsg{Task: task, Project: project} }
		},
		TogglePin: func() (tea.Model, tea.Cmd) { return taskOpsTogglePinned(m) },
		ToggleDim: func() (tea.Model, tea.Cmd) { return taskOpsToggleDimmed(m) },
		ManageTags: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			tags := append([]string{}, task.Tags...)
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: tags}
			}
		},
		ViewComments: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskCommentsMsg{Task: task, Project: project} }
		},
		ViewAttachments: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskAttachmentsMsg{Task: task, Project: project} }
		},
		EditChecklist: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskChecklistMsg{Task: task, Project: project} }
		},
		EditDescription: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowDescriptionEditorMsg{Task: task, Project: project} }
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		ViewDecisions: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskDecisionsMsg{Task: task, Project: project} }
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task != nil {
				m.showDeleteConfirm(task.ShortID)
			}
			return m, nil
		},
	})
	if len(taskPrefixEntries) > 0 && taskPrefixEntries[0].PrefixNode != nil {
		taskPrefixEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, taskPrefixEntries...)

	// --- Stage group: "." prefix, gated on hasItem ---
	stageEntries := keymap.StageGroup(keymap.StageDeps{
		Stages: stages,
		ChangeStage: func(stage string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return taskOpsChangeStage(m, stage)
			}
		},
	})
	if len(stageEntries) > 0 && stageEntries[0].PrefixNode != nil {
		stageEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, stageEntries...)

	// --- Blocker group: "b" prefix, gated on hasItem ---
	blockerEntries := keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowDependencyViewerMsg{Task: task, Project: project} }
		},
		ManageDependencies: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ManageDependenciesMsg{Task: task, Project: project} }
		},
	})
	if len(blockerEntries) > 0 && blockerEntries[0].PrefixNode != nil {
		blockerEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, blockerEntries...)

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

	// --- Common task actions: o, Z, D, e, ?, *, S, O, ctrl+b, ctrl+. ---
	entries = append(entries, keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: project} }
		},
		SnoozeDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks?", m.SelectedCount()))
				return m, nil
			}
			if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
				if entry := m.tabEntries[m.tabs.Active]; !entry.IsBuiltIn {
					m.confirm.Show("delete-smart-list", fmt.Sprintf("Delete smart list %q?", entry.Name))
					return m, nil
				}
			}
			task, _ := m.getTaskRef()
			if task != nil {
				m.showDeleteConfirm(task.ShortID)
			}
			return m, nil
		},
		EditTask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
		},
		ToggleActive: func() (tea.Model, tea.Cmd) { return taskOpsToggleActive(m) },
		EditScore: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowScoreEditorMsg{Task: task, Project: project} }
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

	// Tab navigation
	entries = append(entries,
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+left", &m.tabs, func() {
				if m.activeTabIncludesCompleted() != m.loadedWithCompleted {
					m.loadAllTasks()
				} else {
					m.applyFiltersAndGroup()
				}
				m.clampCursor()
			})
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+right", &m.tabs, func() {
				if m.activeTabIncludesCompleted() != m.loadedWithCompleted {
					m.loadAllTasks()
				} else {
					m.applyFiltersAndGroup()
				}
				m.clampCursor()
			})
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
			if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
				entry := m.tabEntries[m.tabs.Active]
				if !entry.IsBuiltIn {
					sf := m.config.GetSavedFilter(entry.SavedFilterID)
					if sf != nil && sf.Mode == "criteria" {
						filter := *sf
						return m, func() tea.Msg {
							return ShowFilterEditorMsg{Filter: &filter}
						}
					}
				}
			}
			return m, nil
		}),
		keymap.Bind("X", "Delete current smart list tab", "Smart Lists", func() (tea.Model, tea.Cmd) {
			if m.tabs.Active >= 0 && m.tabs.Active < len(m.tabEntries) {
				entry := m.tabEntries[m.tabs.Active]
				if !entry.IsBuiltIn {
					m.confirm.Show("delete-smart-list", fmt.Sprintf("Delete smart list %q?", entry.Name))
				}
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
