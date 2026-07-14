package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (b *outlinerKeymapBuilder) statusEntries() []keymap.Entry {
	m := b.model
	deferRecurrenceChildren := keymap.DeferRecurrenceGroup(
		func() (tea.Model, tea.Cmd) {
			return m.handleDeferRecurrenceOutliner("t")
		},
		func() (tea.Model, tea.Cmd) {
			return m.handleDeferRecurrenceOutliner("3")
		},
		func() (tea.Model, tea.Cmd) {
			return m.handleDeferRecurrenceOutliner("w")
		},
		func() (tea.Model, tea.Cmd) {
			return m.handleDeferRecurrenceOutliner("W")
		},
		func() (tea.Model, tea.Cmd) {
			return m.handleDeferRecurrenceOutliner("m")
		},
	)

	return b.gateOnTask(keymap.StatusGroup(keymap.StatusDeps{
		ChangeState: func(state string) keymap.Action {
			s := models.WorkflowState(state)
			return func() (tea.Model, tea.Cmd) {
				task := b.cursorTask()
				if task == nil {
					return m, nil
				}
				if m.project == nil {
					if model, cmd, handled := m.crossHandleStateMutation(s); handled {
						return model, cmd
					}
					return m, nil
				}
				if m.multiSelectMode && len(m.selectedTasks) > 0 {
					return m.applyBulkStateChange(s)
				}
				return m.applySingleStateTransition(task, s)
			}
		},
		HasRecurrence: func() bool {
			task := b.cursorTask()
			return task != nil && task.Recurrence != nil
		},
		SkipRecurrence: func() (tea.Model, tea.Cmd) {
			return m.handleSkipRecurrenceOutliner()
		},
		DeferChildren: deferRecurrenceChildren,
	}))
}

func (b *outlinerKeymapBuilder) priorityEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handlePriorityPrefixOutliner(key)
			}
		},
	}))
}

func (b *outlinerKeymapBuilder) dueDateEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday: func() (tea.Model, tea.Cmd) {
			return m.handleDueDatePrefixOutliner("t")
		},
		DueTomorrow: func() (tea.Model, tea.Cmd) {
			return m.handleDueDatePrefixOutliner("m")
		},
		DueInAWeek: func() (tea.Model, tea.Cmd) {
			return m.handleDueDatePrefixOutliner("w")
		},
		DueInAMonth: func() (tea.Model, tea.Cmd) {
			return m.handleDueDatePrefixOutliner("n")
		},
		ClearDueDate: func() (tea.Model, tea.Cmd) {
			return m.handleDueDatePrefixOutliner("0")
		},
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			return m.handleDueDatePrefixOutliner("d")
		},
	}))
}

func (b *outlinerKeymapBuilder) snoozeEntries() []keymap.Entry {
	m := b.model
	snoozeDateAction := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			return m.handleSnoozePrefixOutliner(key)
		}
	}

	return b.gateOnTask(keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDateAction("t"),
		SnoozeOneWeek:    snoozeDateAction("w"),
		SnoozeOneMonth:   snoozeDateAction("m"),
		SnoozeNextMonday: snoozeDateAction("n"),
		ClearSnooze:      snoozeDateAction("0"),
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			return m.handleSnoozePrefixOutliner("d")
		},
	}))
}

func (b *outlinerKeymapBuilder) moveEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.MoveGroup(keymap.MoveDeps{
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
				return m.sendTaskToSection(b.cursorTask(), section)
			}
		},
		MoveToTop: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil {
				return m, nil
			}
			return m.moveTaskToPosition(task, true)
		},
		MoveToBottom: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil {
				return m, nil
			}
			return m.moveTaskToPosition(task, false)
		},
	}))
}

func (b *outlinerKeymapBuilder) linkedTaskEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("b", b.cursorTask())
		},
		CreateDependent: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("d", b.cursorTask())
		},
		CreateSubtask: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("s", b.cursorTask())
		},
		CreateFollowUp: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("f", b.cursorTask())
		},
		NavigateLinked: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("n", b.cursorTask())
		},
		CrossProjectLink: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("p", b.cursorTask())
		},
		UnlinkTask: func() (tea.Model, tea.Cmd) {
			return m.handleLinkedTaskPrefixOutliner("x", b.cursorTask())
		},
	}))
}

func (b *outlinerKeymapBuilder) taskPrefixEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("n", b.cursorTask())
		},
		NewTaskWithTemplate: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("N", b.cursorTask())
		},
		RenameTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("r", b.cursorTask())
		},
		ChangeSection: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("s", b.cursorTask())
		},
		OpenEditor: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("o", b.cursorTask())
		},
		TogglePin: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("p", b.cursorTask())
		},
		ToggleDim: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("h", b.cursorTask())
		},
		ManageTags: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("t", b.cursorTask())
		},
		ViewComments: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("c", b.cursorTask())
		},
		ViewAttachments: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("a", b.cursorTask())
		},
		EditChecklist: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("l", b.cursorTask())
		},
		EditDescription: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("e", b.cursorTask())
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("m", b.cursorTask())
		},
		ViewDecisions: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("d", b.cursorTask())
		},
		ParkTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("P", b.cursorTask())
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			return m.handleTaskPrefix("D", b.cursorTask())
		},
	}))
}

func (b *outlinerKeymapBuilder) blockerEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: func() (tea.Model, tea.Cmd) {
			return m.handleBlockerPrefixOutliner("b", b.cursorTask())
		},
		ManageDependencies: func() (tea.Model, tea.Cmd) {
			return m.handleBlockerPrefixOutliner("m", b.cursorTask())
		},
	}))
}

func (b *outlinerKeymapBuilder) filterEntries() []keymap.Entry {
	m := b.model

	entries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("#")
		},
		TogglePriority: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("p")
		},
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("h")
		},
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("i")
		},
		ToggleParked: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("k")
		},
		ClearFilters: func() (tea.Model, tea.Cmd) {
			return m.handleFilterPrefix("x")
		},
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("Ft", key)
			}
		},
		StateKey: func(key string) keymap.Action {
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
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			source := ViewOutliner
			if m.project == nil {
				source = ViewCrossProjectOutliner
			}
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: source} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.handleOutlinerFilterSubKey("FS", key)
			}
		},
		ScoreCustomInput: func() (tea.Model, tea.Cmd) {
			custom := m.runtime.Machine().ScoreFilterCustom
			m.scoreFilterInput.Enter(custom)
			return m, nil
		},
	})
	if len(entries) > 0 && entries[0].PrefixNode != nil {
		extra := []keymap.Entry{
			keymap.Bind("c", "Toggle completed filter", "Filters", func() (tea.Model, tea.Cmd) {
				return m.handleFilterPrefix("c")
			}),
			keymap.Bind("@", "Filter by section", "Filters", func() (tea.Model, tea.Cmd) {
				return m.handleFilterPrefix("@")
			}),
			keymap.Bind("r", "Toggle actionable filter", "Filters", func() (tea.Model, tea.Cmd) {
				return m.handleFilterPrefix("r")
			}),
			keymap.Bind("D", "Choose visible columns", "Filters", func() (tea.Model, tea.Cmd) {
				return m.handleFilterPrefix("D")
			}),
		}
		if m.project == nil {
			extra = append(extra, keymap.Bind("P", "Filter by projects", "Filters", func() (tea.Model, tea.Cmd) {
				return m.handleFilterPrefix("P")
			}))
		}
		extra = append(extra, savedFilterPrefixEntry(m))
		entries[0].PrefixNode.Children = append(entries[0].PrefixNode.Children, extra...)
	}
	return entries
}

func (b *outlinerKeymapBuilder) sortEntries() []keymap.Entry {
	m := b.model

	return keymap.SortGroup(keymap.SortDeps{
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
	})
}
