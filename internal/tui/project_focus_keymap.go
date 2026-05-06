package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for ProjectFocusViewModel.
func (m *ProjectFocusViewModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.cfg != nil {
		stages = m.cfg.Stages
	}

	hasItem := func() bool { return m.getTaskAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			tasks := m.filteredTasks()
			if m.cursor < len(tasks)-1 {
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
			tasks := m.filteredTasks()
			if len(tasks) > 0 {
				m.cursor = len(tasks) - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filteredTasks()), -HalfPage(m.ContentHeight()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filteredTasks()), HalfPage(m.ContentHeight()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)

	// --- Goto group: g prefix (gg = top only, no gp) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			// No gp action for this view
			return m, nil
		},
	})...)

	// --- z prefix: no-ops for zR, zM ---
	entries = append(entries, keymap.Prefix("z", "View", "View", []keymap.Entry{
		keymap.Bind("R", "Expand all (no-op)", "View", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("M", "Collapse all (no-op)", "View", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	}))

	// --- Status group: "," prefix, gated on hasItem ---
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus: func(status string) keymap.Action {
			s := models.TaskStatus(status)
			return func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(m, s) }
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
			return func() (tea.Model, tea.Cmd) { return taskOpsChangePriority(m, key) }
		},
	})
	if len(priorityEntries) > 0 && priorityEntries[0].PrefixNode != nil {
		priorityEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, priorityEntries...)

	// --- Due date group: "d" prefix, gated on hasItem ---
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:     func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(m, 0) },
		DueTomorrow:  func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(m, 1) },
		DueInAWeek:   func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(m, 7) },
		DueInAMonth:  func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(m, 30) },
		ClearDueDate: func() (tea.Model, tea.Cmd) { return taskOpsClearDueDate(m) },
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
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return taskOpsSectionMove(m, section) }
		},
		MoveToTop:    func() (tea.Model, tea.Cmd) { return m, nil },
		MoveToBottom: func() (tea.Model, tea.Cmd) { return m, nil },
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
		DeleteTask: func() (tea.Model, tea.Cmd) {
			// No delete in this view
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
			return func() (tea.Model, tea.Cmd) { return taskOpsChangeStage(m, stage) }
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

	// --- View-specific direct bindings ---

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

	// Tab / sub-tab navigation
	entries = append(entries,
		keymap.Bind("tab", "Next sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
				m.clampCursor()
			}
			return m, nil
		}),
		keymap.Bind("shift+right", "Next sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, 1)
				m.clampCursor()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
				m.clampCursor()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+left", "Previous sub-tab", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.subTabNames) > 1 {
				CycleTab(&m.subTab, len(m.subTabNames)+1, -1)
				m.clampCursor()
			}
			return m, nil
		}, keymap.AsHidden()),
	)

	// Enter: close and return to project tree
	entries = append(entries,
		keymap.Bind("enter", "Close and return to project tree", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CloseProjectFocusViewMsg{} }
		}),
	)

	// Toggle active flag
	entries = append(entries,
		keymap.Bind("*", "Remove from focus", "Actions", func() (tea.Model, tea.Cmd) {
			return m.toggleActive()
		}),
	)

	// Edit task
	entries = append(entries,
		keymap.Bind("e", "Edit task", "Actions", func() (tea.Model, tea.Cmd) {
			if task := m.getTaskAtCursor(); task != nil {
				return m, func() tea.Msg { return EditTaskMsg{Task: task} }
			}
			return m, nil
		}),
	)

	// Refresh
	entries = append(entries,
		keymap.Bind("r", "Refresh", "View", func() (tea.Model, tea.Cmd) {
			_ = defaultPersister.Load(m.project)
			m.loadTasks()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// Priority direct keys: 1-5
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				return m.changeTaskPriority(key)
			}, keymap.AsHidden()),
		)
	}

	return &keymap.KeyMap{
		ViewName: "Project Focus",
		Entries:  entries,
	}
}
