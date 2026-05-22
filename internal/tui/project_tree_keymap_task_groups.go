package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (b *projectTreeKeymapBuilder) statusEntries() []keymap.Entry {
	m := b.model
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

	return b.gateOnTask(keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus: func(status string) keymap.Action {
			s := models.TaskStatus(status)
			return func() (tea.Model, tea.Cmd) {
				_, cmd := m.handleStatusTransition(s, b.visible())
				return m, cmd
			}
		},
		HasRecurrence: func() bool {
			task := b.cursorTask()
			return task != nil && task.Recurrence != nil
		},
		SkipRecurrence: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSkipRecurrence(m)
			return m, cmd
		},
		DeferChildren: deferRecurrenceChildren,
	}))
}

func (b *projectTreeKeymapBuilder) priorityEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				cmd, _ := handlePriorityPrefix(key, m)
				return m, cmd
			}
		},
	}))
}

func (b *projectTreeKeymapBuilder) dueDateEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.DueDateGroup(keymap.DueDateDeps{
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
	}))
}

func (b *projectTreeKeymapBuilder) snoozeEntries() []keymap.Entry {
	m := b.model
	snoozeDateAction := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSnoozePrefix(key, m)
			return m, cmd
		}
	}

	return b.gateOnTask(keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDateAction("t"),
		SnoozeOneWeek:    snoozeDateAction("w"),
		SnoozeOneMonth:   snoozeDateAction("m"),
		SnoozeNextMonday: snoozeDateAction("n"),
		ClearSnooze:      snoozeDateAction("0"),
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleSnoozePrefix("d", m)
			return m, cmd
		},
	}))
}

func (b *projectTreeKeymapBuilder) moveEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				_, cmd := m.sendTasksToSection(section, b.visible())
				return m, cmd
			}
		},
		MoveToTop:    func() (tea.Model, tea.Cmd) { return m.moveTaskToTop(b.visible()) },
		MoveToBottom: func() (tea.Model, tea.Cmd) { return m.moveTaskToBottom(b.visible()) },
	}))
}

func (b *projectTreeKeymapBuilder) linkedTaskEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
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
		UnlinkTask: func() (tea.Model, tea.Cmd) {
			node := b.currentNode()
			if node == nil {
				return m, nil
			}

			handled, cmd := handleLinkedTaskPrefixKey("x", node.Task, m.project, true, true)
			if handled {
				return m, cmd
			}

			return m, nil
		},
	}))
}

func (b *projectTreeKeymapBuilder) taskPrefixEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
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
		TogglePin: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleTogglePinnedKey(m)
			return m, cmd
		},
		ToggleDim: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleToggleDimmedKey(m)
			return m, cmd
		},
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
			handleDeleteConfirmKey(m)
			return m, nil
		},
	}))
}

func (b *projectTreeKeymapBuilder) stageEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.StageGroup(keymap.StageDeps{
		Stages: b.stages,
		ChangeStage: func(stage string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				for _, key := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
					if mappedStage := StageForPrefixKey(key, b.stages); mappedStage != nil && *mappedStage == stage {
						cmd, _ := handleStagePrefix(key, m, b.stages)
						return m, cmd
					}
				}

				return m, nil
			}
		},
	}))
}

func (b *projectTreeKeymapBuilder) blockerEntries() []keymap.Entry {
	m := b.model

	return b.gateOnTask(keymap.BlockerGroup(keymap.BlockerDeps{
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
	}))
}

func (b *projectTreeKeymapBuilder) filterEntries() []keymap.Entry {
	m := b.model

	entries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Ft", key) }
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fd", key) }
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fa", key) }
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix(prefix, key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: ViewProjectTree} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("F.", key) }
		},
		StageFilterNames: b.stages,
	})
	if len(entries) > 0 && entries[0].PrefixNode != nil {
		entries[0].PrefixNode.Children = append(entries[0].PrefixNode.Children, savedFilterPrefixEntry(m))
	}
	return entries
}
