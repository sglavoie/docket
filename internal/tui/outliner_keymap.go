package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for OutlinerModel.
func (m *OutlinerModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	cursorTask := func() *models.Task {
		if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			if !row.IsSectionHeader && !row.IsSeparator {
				return row.task
			}
		}
		return nil
	}
	hasTask := func() bool { return cursorTask() != nil }
	onHeader := func() bool {
		if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
			return false
		}
		return m.rows[m.cursor].IsSectionHeader || m.rows[m.cursor].IsSeparator
	}

	var entries []keymap.Entry

	// --- List Navigation ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("j", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipSeparators(prev, cmd)
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("k", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipSeparators(prev, cmd)
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("G", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipSeparators(prev, cmd)
			}
			return m, nil
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("ctrl+u", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipSeparators(prev, cmd)
			}
			return m, nil
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("ctrl+d", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipSeparators(prev, cmd)
			}
			return m, nil
		},
	})...)

	// --- CommonPrimaryViewGroup ---
	entries = append(entries, keymap.CommonPrimaryViewGroup(keymap.CommonPrimaryViewDeps{
		OpenProjectTree: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectTreeMsg{Project: m.project} }
		},
		OpenOutliner: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowOutlinerMsg{Project: m.project} }
		},
		NextProject: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CycleProjectMsg{Forward: true} }
		},
		PrevProject: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CycleProjectMsg{Forward: false} }
		},
		OpenCycleManager: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectCycleManagerMsg{} }
		},
		ProjectBlockers: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowBlockersMsg{Project: project} }
		},
		PriorityView: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowPriorityViewMsg{Project: project} }
		},
		AddToPlan: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusBoardMsg{} }
		},
		InboxProcessor: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowInboxProcessorMsg{Project: project} }
		},
		QuickCapture: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{Project: m.project, AllProjects: m.config.GetActiveProjects()}
			}
		},
		BulkCapture: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowBulkCaptureMsg{Project: m.project} }
		},
		JumpToTask: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowTaskJumperMsg{Project: m.project} }
		},
		OpenNotesView: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowNotesMsg{Project: project} }
		},
		CreateNote: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowNoteCreatorMsg{Project: project} }
		},
		ReloadFromDisk: func() (tea.Model, tea.Cmd) {
			return m, m.reloadFromDisk()
		},
		Redo: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				model, cmd, _ := m.crossHandleRedo("ctrl+r")
				return model, cmd
			}
			return m, m.performRedo()
		},
		Undo: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				model, cmd, _ := m.crossHandleUndo("u")
				return model, cmd
			}
			return m, m.performUndo()
		},
		EditLastModified: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			recent := storage.GetRecentTasks(m.project.Tasks, 1)
			if len(recent) > 0 {
				task := recent[0]
				return m, func() tea.Msg { return EditTaskMsg{Task: task} }
			}
			return m, nil
		},
		MoveReparent: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleResolvedTaskKey("ctrl+m", m, m.project)
			return m, cmd
		},
	})...)

	// --- Goto group: g prefix ---
	entries = append(entries, keymap.Prefix("g", "Go…", "Navigation", []keymap.Entry{
		keymap.Bind("g", "Go to top", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.JumpToTop()
		}),
		keymap.Bind("h", "View task activity log", "Task Operations", func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			if m.project == nil || task == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return OpenTaskActivityMsg{Task: task, Project: project} }
		}),
		keymap.Bind("l", "Project recent activity", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowProjectRecentActivityMsg{Project: project} }
		}),
		keymap.Bind("d", "Cycle due date filter", "Filters", func() (tea.Model, tea.Cmd) {
			m.cycleDueDateFilter()
			m.rebuildRows()
			return m, nil
		}),
		keymap.Bind("z", "Jump to Snoozed perspective", "Navigation", func() (tea.Model, tea.Cmd) {
			m.activePerspective = PerspectiveSnoozed
			m.rebuildRows()
			return m, nil
		}),
		keymap.Bind("o", "Drill down to project outliner", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m.crossHandleGoDrillDown()
			}
			return m, nil
		}),
		keymap.Bind("k", "Open kanban view", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
			}
			project := m.project
			return m, func() tea.Msg { return ShowKanbanMsg{Project: project} }
		}),
		keymap.Bind("X", "Deselect all (multi-select)", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAllVisible()
			}
			return m, nil
		}),
	}))

	// --- Perspective prefix: P ---
	perspectiveChildren := []keymap.Entry{
		keymap.Bind("1", "All perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("1")
		}),
		keymap.Bind("2", "Active perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("2")
		}),
		keymap.Bind("3", "Next perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("3")
		}),
		keymap.Bind("4", "Inbox perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("4")
		}),
		keymap.Bind("5", "Standby perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("5")
		}),
		keymap.Bind("6", "Waiting perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("6")
		}),
		keymap.Bind("7", "Someday perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("7")
		}),
		keymap.Bind("8", "Completed perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("8")
		}),
		keymap.Bind("9", "Review perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("9")
		}),
		keymap.Bind("0", "Snoozed perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
			return m.handlePerspectivePrefix("0")
		}),
	}
	entries = append(entries, keymap.Prefix("P", "Switch perspective", "Perspectives", perspectiveChildren))

	// --- FoldGroup: z prefix ---
	entries = append(entries, keymap.FoldGroup(keymap.FoldDeps{
		ToggleFold: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("a", task)
		},
		ExpandFold: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("o", task)
		},
		CollapseFold: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("c", task)
		},
		ExpandAll: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("R", task)
		},
		CollapseAll: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("M", task)
		},
		FocusSubtree: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("v", task)
		},
		ToggleInlinePreview: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			return m.handleFoldPrefix("p", task)
		},
	})...)

	// --- StatusGroup: , prefix ---
	deferRecurrenceChildren := keymap.DeferRecurrenceGroup(
		func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDeferRecurrence("t", m)
			return m, cmd
		},
		func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDeferRecurrence("3", m)
			return m, cmd
		},
		func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDeferRecurrence("w", m)
			return m, cmd
		},
		func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDeferRecurrence("W", m)
			return m, cmd
		},
		func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDeferRecurrence("m", m)
			return m, cmd
		},
	)
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus: func(status string) keymap.Action {
			s := models.TaskStatus(status)
			return func() (tea.Model, tea.Cmd) {
				task := cursorTask()
				if task == nil {
					return m, nil
				}
				if m.project == nil {
					if model, cmd, handled := m.crossHandleStatusMutation(string(s)); handled {
						return model, cmd
					}
					return m, nil
				}
				if m.multiSelectMode && len(m.selectedTasks) > 0 {
					return m.applyBulkStatusChange(s)
				}
				pf := m.workflowPersistFn()
				result := applyStatusChange(pf, m.project, task.ID, s)
				if result.Err != nil {
					return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
				}
				m.setStatus(result.StatusMessage)
				m.rebuildRows()
				return m, result.Cmd
			}
		},
		HasRecurrence: func() bool {
			task := cursorTask()
			return task != nil && task.Recurrence != nil
		},
		SkipRecurrence: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSkipRecurrence(m)
			return m, cmd
		},
		DeferChildren: deferRecurrenceChildren,
	})
	if len(statusEntries) > 0 && statusEntries[0].PrefixNode != nil {
		statusEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, statusEntries...)

	// --- PriorityGroup: ! prefix ---
	priorityEntries := keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				cmd, _ := handlePriorityPrefix(key, m)
				return m, cmd
			}
		},
	})
	if len(priorityEntries) > 0 && priorityEntries[0].PrefixNode != nil {
		priorityEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, priorityEntries...)

	// --- DueDateGroup: d prefix ---
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDueDatePrefix("t", m)
			return m, cmd
		},
		DueTomorrow: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDueDatePrefix("m", m)
			return m, cmd
		},
		DueInAWeek: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDueDatePrefix("w", m)
			return m, cmd
		},
		DueInAMonth: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDueDatePrefix("n", m)
			return m, cmd
		},
		ClearDueDate: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDueDatePrefix("0", m)
			return m, cmd
		},
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleDueDatePrefix("d", m)
			return m, cmd
		},
	})
	if len(dueDateEntries) > 0 && dueDateEntries[0].PrefixNode != nil {
		dueDateEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, dueDateEntries...)

	// --- SnoozeGroup: s prefix ---
	snoozeDateAction := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSnoozePrefix(key, m)
			return m, cmd
		}
	}
	snoozeEntries := keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDateAction("t"),
		SnoozeOneWeek:    snoozeDateAction("w"),
		SnoozeOneMonth:   snoozeDateAction("m"),
		SnoozeNextMonday: snoozeDateAction("n"),
		ClearSnooze:      snoozeDateAction("0"),
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSnoozePrefix("d", m)
			return m, cmd
		},
	})
	if len(snoozeEntries) > 0 && snoozeEntries[0].PrefixNode != nil {
		snoozeEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, snoozeEntries...)

	// --- MoveGroup: m prefix ---
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.project == nil {
					if m.multiSelectMode && len(m.selectedTasks) > 0 {
						m.setStatus("bulk section move not supported in cross-project mode")
						return m, nil
					}
					return m.crossSendTaskToSection(section)
				}
				if m.multiSelectMode && len(m.selectedTasks) > 0 {
					return m.sendTasksToSectionBulk(section)
				}
				return m.sendTaskToSection(cursorTask(), section)
			}
		},
		MoveToTop: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			if task == nil {
				return m, nil
			}
			return m.moveTaskToPosition(task, true)
		},
		MoveToBottom: func() (tea.Model, tea.Cmd) {
			task := cursorTask()
			if task == nil {
				return m, nil
			}
			return m.moveTaskToPosition(task, false)
		},
	})
	if len(moveEntries) > 0 && moveEntries[0].PrefixNode != nil {
		moveEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, moveEntries...)

	// --- LinkedTaskGroup: c prefix ---
	linkedEntries := keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("b", cursorTask())
		},
		CreateDependent: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("d", cursorTask())
		},
		CreateSubtask: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("s", cursorTask())
		},
		CreateFollowUp: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("f", cursorTask())
		},
		NavigateLinked: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("n", cursorTask())
		},
		CrossProjectLink: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("p", cursorTask())
		},
		UnlinkTask: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("x", cursorTask())
		},
	})
	if len(linkedEntries) > 0 && linkedEntries[0].PrefixNode != nil {
		linkedEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, linkedEntries...)

	// --- TaskPrefixGroup: t prefix ---
	taskPrefixEntries := keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("n", cursorTask())
		},
		NewTaskWithTemplate: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("N", cursorTask())
		},
		RenameTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("r", cursorTask())
		},
		ChangeSection: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("s", cursorTask())
		},
		OpenEditor: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("o", cursorTask())
		},
		TogglePin: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("p", cursorTask())
		},
		ToggleDim: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("d", cursorTask())
		},
		ManageTags: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("t", cursorTask())
		},
		ViewComments: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("c", cursorTask())
		},
		ViewAttachments: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("a", cursorTask())
		},
		EditChecklist: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("l", cursorTask())
		},
		EditDescription: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("e", cursorTask())
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("m", cursorTask())
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("D", cursorTask())
		},
	})
	if len(taskPrefixEntries) > 0 && taskPrefixEntries[0].PrefixNode != nil {
		taskPrefixEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, taskPrefixEntries...)

	// --- StageGroup: . prefix ---
	stageEntries := keymap.StageGroup(keymap.StageDeps{
		Stages: stages,
		ChangeStage: func(stage string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				for _, k := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
					if s := StageForPrefixKey(k, stages); s != nil && *s == stage {
						return m.handleStagePrefixOutliner(k)
					}
				}
				return m, nil
			}
		},
	})
	if len(stageEntries) > 0 && stageEntries[0].PrefixNode != nil {
		stageEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, stageEntries...)

	// --- BlockerGroup: b prefix ---
	blockerEntries := keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: func() (tea.Model, tea.Cmd) {
			return m.handleBlockerPrefixOutliner("b", cursorTask())
		},
		ManageDependencies: func() (tea.Model, tea.Cmd) {
			return m.handleBlockerPrefixOutliner("m", cursorTask())
		},
	})
	if len(blockerEntries) > 0 && blockerEntries[0].PrefixNode != nil {
		blockerEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, blockerEntries...)

	// --- FilterGroup: F prefix ---
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("#")
		},
		TogglePriority: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("p")
		},
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("h")
		},
		ClearFilters: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("x")
		},
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("Ft", key)
			}
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("Fs", key)
			}
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("Fd", key)
			}
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("Fa", key)
			}
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey(prefix, key)
			}
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("FS", key)
			}
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("F.", key)
			}
		},
		StageFilterNames: stages,
	})...)

	// --- SortGroup: 1-5, 0 ---
	entries = append(entries, keymap.SortGroup(keymap.SortDeps{
		SortByPriority: func() (tea.Model, tea.Cmd) {
			m.clearActiveSavedFilter()
			handleSortKey("1", &m.sort, func() { m.rebuildRows() }, func(SortField) { m.rebuildRows() })
			return m, nil
		},
		SortByDueDate: func() (tea.Model, tea.Cmd) {
			m.clearActiveSavedFilter()
			handleSortKey("2", &m.sort, func() { m.rebuildRows() }, func(SortField) { m.rebuildRows() })
			return m, nil
		},
		SortByCreated: func() (tea.Model, tea.Cmd) {
			m.clearActiveSavedFilter()
			handleSortKey("3", &m.sort, func() { m.rebuildRows() }, func(SortField) { m.rebuildRows() })
			return m, nil
		},
		SortByStatus: func() (tea.Model, tea.Cmd) {
			m.clearActiveSavedFilter()
			handleSortKey("4", &m.sort, func() { m.rebuildRows() }, func(SortField) { m.rebuildRows() })
			return m, nil
		},
		SortByScore: func() (tea.Model, tea.Cmd) {
			m.clearActiveSavedFilter()
			handleSortKey("5", &m.sort, func() { m.rebuildRows() }, func(SortField) { m.rebuildRows() })
			return m, nil
		},
		ResetSort: func() (tea.Model, tea.Cmd) {
			m.clearActiveSavedFilter()
			handleSortKey("0", &m.sort, func() { m.rebuildRows() }, func(SortField) { m.rebuildRows() })
			return m, nil
		},
	})...)

	// --- TypeChange group: < > H L ---
	entries = append(entries,
		keymap.Bind("<", "Promote task type", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("<")
			return model, cmd
		}),
		keymap.Bind(">", "Demote task type", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey(">")
			return model, cmd
		}),
		keymap.Bind("H", "Cycle type left", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("H")
			return model, cmd
		}),
		keymap.Bind("L", "Cycle type right", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("L")
			return model, cmd
		}),
	)

	// --- Perspective cycling (h/l) ---
	entries = append(entries,
		keymap.Bind("h", "Cycle perspective left", "Perspectives", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m.crossHandlePerspectiveCycle("h")
			}
			perspCount := len(PerspectivePresets())
			m.activePerspective = (m.activePerspective + perspCount - 1) % perspCount
			m.rebuildRows()
			return m, nil
		}),
		keymap.Bind("l", "Cycle perspective right", "Perspectives", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m.crossHandlePerspectiveCycle("l")
			}
			perspCount := len(PerspectivePresets())
			m.activePerspective = (m.activePerspective + 1) % perspCount
			m.rebuildRows()
			return m, nil
		}),
	)

	// --- Section navigation: n/N ---
	entries = append(entries,
		keymap.Bind("n", "Jump to next section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.jumpToNextSection(1)
		}),
		keymap.Bind("N", "Jump to previous section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.jumpToNextSection(-1)
		}),
	)

	// --- Navigation keys ---
	entries = append(entries,
		keymap.Bind("enter", "Zoom into parent / open leaf", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleNavigationKey("enter")
			return model, cmd
		}),
		keymap.Bind("backspace", "Zoom out one level", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleNavigationKey("backspace")
			return model, cmd
		}),
		keymap.Bind("tab", "Cycle sidebar state", "Sidebar", func() (tea.Model, tea.Cmd) {
			task := m.SidebarSelectedTask()
			m.sidebar.CycleState(task)
			if m.sidebar.IsFocused() && task != nil {
				if proj := m.SidebarProject(); proj != nil {
					sections := sidebarGetAvailableSections(proj, task)
					if len(sections) > 0 {
						m.sidebar.SetDependencySection(sections[0])
					}
				}
			}
			return m, nil
		}),
	)

	// --- Task mutation keys: J/K/right/left ---
	entries = append(entries,
		keymap.Bind("J", "Move task down", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("J", onHeader())
			return model, cmd
		}),
		keymap.Bind("K", "Move task up", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("K", onHeader())
			return model, cmd
		}),
		keymap.Bind("right", "Indent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("right", onHeader())
			return model, cmd
		}, keymap.AsHidden()),
		keymap.Bind("left", "Outdent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("left", onHeader())
			return model, cmd
		}, keymap.AsHidden()),
	)

	// --- Task view keys ---
	entries = append(entries,
		keymap.Bind("o", "View task details", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("o", onHeader())
			return model, cmd
		}),
		keymap.Bind("e", "Edit task", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("e", onHeader())
			return model, cmd
		}),
		keymap.Bind("E", "Quick edit (inline)", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("E", onHeader())
			return model, cmd
		}),
		keymap.Bind("D", "Delete task", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("D", onHeader())
			return model, cmd
		}),
		keymap.Bind("ctrl+f", "Filter summary", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("ctrl+f", onHeader())
			return model, cmd
		}),
	)

	// --- Utility keys ---
	entries = append(entries,
		keymap.Bind("v", "Toggle multi-select mode", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) &&
					!m.rows[m.cursor].IsSectionHeader && !m.rows[m.cursor].IsSeparator && m.rows[m.cursor].task != nil {
					m.ToggleSelection(m.rows[m.cursor].task.ID)
				}
			}
			return m, nil
		}),
		keymap.Bind("a", "Select all visible", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.selectAllVisible()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("A", "Deselect all / archive / all-tasks", "Views", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAllVisible()
				return m, nil
			}
			model, cmd, _ := m.handleUtilityKey("A")
			return model, cmd
		}),
		keymap.Bind("T", "Tag filter selector", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("T")
			return model, cmd
		}),
		keymap.Bind("C", "Context filter", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("C")
			return model, cmd
		}),
		keymap.Bind("f", "Focus view (project)", "Views", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("f")
			return model, cmd
		}),
		keymap.Bind("*", "Toggle active flag", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("*")
			return model, cmd
		}),
		keymap.Bind("shift+left", "Previous custom section tab", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("shift+left")
			return model, cmd
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next custom section tab", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("shift+right")
			return model, cmd
		}, keymap.AsHidden()),
		keymap.Bind("ctrl+a", "Archive/unarchive project", "Project", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+a")
			return model, cmd
		}),
		keymap.Bind("ctrl+t", "Tag manager", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+t")
			return model, cmd
		}),
		keymap.Bind("ctrl+s", "Section manager", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+s")
			return model, cmd
		}),
		keymap.Bind("ctrl+v", "Create task from clipboard", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+v")
			return model, cmd
		}),
		keymap.Bind("ctrl+e", "Export project to Markdown", "Project", func() (tea.Model, tea.Cmd) {
			project := m.project
			return m, func() tea.Msg { return ExportProjectMsg{Project: project} }
		}),
		keymap.Bind("/", "Search tasks", "Filters", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
		keymap.Bind("x", "Unlink task (sidebar)", "Sidebar", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("S", "Edit score dimensions", "Scoring", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: cursorTask(), Project: m.project}
			}
		}),
	)

	// Saved view filter (Fv prefix - added as part of F prefix in FilterGroup, but declare here for Fvs/Fvl/Fvd)
	// These are handled by handleSavedFilterPrefix which uses handleSavedFilterPrefixKey
	// The Fv sub-prefix is declared within FilterGroup via the keymap, so we add the Fv children here
	// by adding an Fv prefix entry after the FilterGroup.
	// Actually, FilterGroup doesn't include Fv - let's add it manually under F children.
	// For now, declare the Fv prefix as a top-level entry that the Dispatcher understands:
	// (This needs to be a child of the F prefix node; since we can't do that easily here,
	// the Dispatcher will handle "F" -> "v" -> children)

	return &keymap.KeyMap{
		ViewName: "Outliner",
		Entries:  entries,
	}
}

// handleOutlinerFilterSubKey handles filter sub-prefix keys for the outliner.
func (m *OutlinerModel) handleOutlinerFilterSubKey(prefix, key string) (tea.Model, tea.Cmd) {
	if prefix == "FS" && key == "C" {
		var custom *float64
		if m.config != nil {
			custom = m.config.ScoreFilterCustom
		}
		m.scoreFilterInput.Enter(custom)
		return m, nil
	}
	if dispatchFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.config, func(p string) {
		if m.Dispatcher != nil {
			m.Dispatcher.SetPendingPrefix(p)
		} else {
			m.pendingPrefix = p
		}
	}) {
		m.rebuildRows()
	}
	return m, nil
}
