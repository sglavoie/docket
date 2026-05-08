package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for ProjectTreeModel.
func (m *ProjectTreeModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	visible := func() []*TreeNode { return m.visibleNodes() }
	cursorTask := func() *models.Task {
		v := visible()
		if CursorInBounds(m.cursor, len(v)) {
			return v[m.cursor].Task
		}
		return nil
	}
	hasTask := func() bool { return cursorTask() != nil }

	var entries []keymap.Entry

	// CommonPrimaryViewGroup: ctrl+p, ctrl+o, `, ~, @, B, p, w, W, i, I, \, {, }, R, ctrl+r, u, ctrl+l, ctrl+m
	entries = append(entries, keymap.CommonPrimaryViewGroup(keymap.CommonPrimaryViewDeps{
		OpenProjectTree: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectTreeMsg{Project: m.getProject()} }
		},
		OpenOutliner: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowOutlinerMsg{Project: m.getProject()} }
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
			project := m.getProject()
			if project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowBlockersMsg{Project: project} }
		},
		PriorityView: func() (tea.Model, tea.Cmd) {
			project := m.getProject()
			if project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowPriorityViewMsg{Project: project} }
		},
		AddToPlan: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusBoardMsg{} }
		},
		InboxProcessor: func() (tea.Model, tea.Cmd) {
			project := m.getProject()
			if project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowInboxProcessorMsg{Project: project} }
		},
		QuickCapture: func() (tea.Model, tea.Cmd) {
			cfg := m.getConfig()
			project := m.getProject()
			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{Project: project, AllProjects: cfg.Projects}
			}
		},
		BulkCapture: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowBulkCaptureMsg{Project: m.getProject()} }
		},
		JumpToTask: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowTaskJumperMsg{Project: m.getProject()} }
		},
		OpenNotesView: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowNotesMsg{Project: m.getProject()} }
		},
		CreateNote: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowNoteCreatorMsg{Project: m.getProject()} }
		},
		ReloadFromDisk: func() (tea.Model, tea.Cmd) {
			return m, m.reloadFromDisk()
		},
		Redo: func() (tea.Model, tea.Cmd) {
			return m, m.performRedo()
		},
		EditLastModified: func() (tea.Model, tea.Cmd) {
			project := m.getProject()
			if project == nil {
				return m, nil
			}
			recent := storage.GetRecentTasks(project.Tasks, 1)
			if len(recent) > 0 {
				return m, func() tea.Msg { return EditTaskMsg{Task: recent[0]} }
			}
			return m, nil
		},
		Undo: func() (tea.Model, tea.Cmd) {
			return m, m.performUndo()
		},
		MoveReparent: func() (tea.Model, tea.Cmd) {
			task, project := resolveTaskAndProject(m, m.getProject())
			if task == nil || project == nil {
				return m, nil
			}
			validParents := buildReparentValidParents(project, task)
			taskID := task.ID
			var currentParentID *string
			if task.ParentID != nil {
				id := *task.ParentID
				currentParentID = &id
			}
			return m, func() tea.Msg {
				return OpenParentSelectorMsg{
					ValidParents:   validParents,
					CurrentID:      currentParentID,
					AllowNone:      true,
					Source:         ViewProjectTree,
					ReparentTaskID: taskID,
				}
			}
		},
	})...)

	// S (score editor), ctrl+. (quick note) — task-specific shared actions
	entries = append(entries,
		keymap.Bind("S", "Edit score dimensions", "Scoring", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleResolvedTaskKey("S", m, m.getProject())
			return m, cmd
		}),
		keymap.Bind("ctrl+.", "Quick note on task", "Task Operations", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleResolvedTaskKey("ctrl+.", m, m.getProject())
			return m, cmd
		}),
	)

	// Navigation: j, k, G, h, l, n, N
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			v := visible()
			if m.cursor < len(v)-1 {
				m.cursor++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("G", "Jump to last", "Navigation", func() (tea.Model, tea.Cmd) {
			v := visible()
			old := m.cursor
			if len(v) > 0 {
				m.cursor = len(v) - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		}),
		keymap.Bind("h", "Previous section", "Navigation", func() (tea.Model, tea.Cmd) {
			if !m.sidebar.IsFocused() {
				m.clearActiveSavedFilter()
				m.cycleViewMode(-1)
			}
			return m, nil
		}),
		keymap.Bind("l", "Next section", "Navigation", func() (tea.Model, tea.Cmd) {
			if !m.sidebar.IsFocused() {
				m.clearActiveSavedFilter()
				m.cycleViewMode(1)
			}
			return m, nil
		}),
		keymap.Bind("n", "Next root task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToNext(rootNodeIndices(visible()))
		}),
		keymap.Bind("N", "Previous root task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToPrev(rootNodeIndices(visible()))
		}),
	)

	// Goto group: g prefix
	setViewMode := func(vm ViewMode) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			m.viewMode = vm
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		}
	}
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: nil, // already in project tree
		Extra: []keymap.Entry{
			keymap.Bind("v", "Cycle view mode", "Views", func() (tea.Model, tea.Cmd) {
				m.viewMode = (m.viewMode + 1) % (ViewModeAll + 1)
				m.updateVisibility()
				m.pruneInvisibleSelections()
				m.clampCursor()
				return m, nil
			}),
			keymap.Bind("a", "Go to Active view", "Views", setViewMode(ViewModeActive)),
			keymap.Bind("n", "Go to Next view", "Views", setViewMode(ViewModeNext)),
			keymap.Bind("i", "Go to Inbox view", "Views", setViewMode(ViewModeInbox)),
			keymap.Bind("w", "Go to Waiting view", "Views", setViewMode(ViewModeWaiting)),
			keymap.Bind("s", "Go to Someday view", "Views", setViewMode(ViewModeSomeday)),
			keymap.Bind("c", "Go to Completed view", "Views", setViewMode(ViewModeCompleted)),
			keymap.Bind("z", "Go to Snoozed view", "Views", setViewMode(ViewModeSnoozed)),
			keymap.Bind("*", "Go to All view", "Views", setViewMode(ViewModeAll)),
			keymap.Bind("r", "Go to Review view", "Views", setViewMode(ViewModeReview)),
			keymap.Bind("S", "Go to Standby view", "Views", setViewMode(ViewModeStandby)),
			keymap.Bind("o", "Jump to context node's section", "Navigation", func() (tea.Model, tea.Cmd) {
				v := visible()
				if CursorInBounds(m.cursor, len(v)) {
					node := v[m.cursor]
					if node.IsContextNode {
						task := node.Task
						if task.IsComplete() {
							m.viewMode = ViewModeCompleted
						} else {
							switch task.Section {
							case models.SectionNext:
								m.viewMode = ViewModeNext
							case models.SectionInbox:
								m.viewMode = ViewModeInbox
							case models.SectionWaiting:
								m.viewMode = ViewModeWaiting
							case models.SectionSomeday:
								m.viewMode = ViewModeSomeday
							case models.SectionReview:
								m.viewMode = ViewModeReview
							case models.SectionStandby:
								m.viewMode = ViewModeStandby
							default:
								m.viewMode = ViewModeActive
							}
						}
						m.buildTree()
						return m, m.NavigateToTask(task)
					}
				}
				return m, nil
			}),
			keymap.Bind("d", "Cycle due date filter", "Filters", func() (tea.Model, tea.Cmd) {
				m.filter.DueDate = (m.filter.DueDate + 1) % 5
				m.updateVisibility()
				m.pruneInvisibleSelections()
				m.clampCursor()
				return m, nil
			}),
			keymap.Bind("X", "Deselect all", "Multi-select", func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode {
					for _, node := range m.visibleNodes() {
						delete(m.selectedTasks, node.Task.ID)
					}
				}
				return m, nil
			}),
			keymap.Bind("h", "View task activity log", "Task Operations", func() (tea.Model, tea.Cmd) {
				v := visible()
				if CursorInBounds(m.cursor, len(v)) {
					task := v[m.cursor].Task
					project := m.project
					return m, func() tea.Msg { return OpenTaskActivityMsg{Task: task, Project: project} }
				}
				return m, nil
			}),
			keymap.Bind("l", "Project recent activity", "Navigation", func() (tea.Model, tea.Cmd) {
				project := m.project
				return m, func() tea.Msg { return ShowProjectRecentActivityMsg{Project: project} }
			}),
			keymap.Bind("k", "Open kanban view", "Navigation", func() (tea.Model, tea.Cmd) {
				project := m.project
				return m, func() tea.Msg { return ShowKanbanMsg{Project: project} }
			}),
		},
	})...)

	// z prefix: FoldGroup
	entries = append(entries, keymap.FoldGroup(keymap.FoldDeps{
		ToggleFold: func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				node := v[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
			return m, nil
		},
		ExpandFold: func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				node := v[m.cursor]
				node.Expanded = true
				m.updateVisibility()
			}
			return m, nil
		},
		CollapseFold: func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				node := v[m.cursor]
				node.Expanded = false
				m.updateVisibility()
			}
			return m, nil
		},
		ExpandAll: func() (tea.Model, tea.Cmd) {
			for _, node := range m.nodes {
				node.Expanded = true
			}
			m.updateVisibility()
			return m, nil
		},
		CollapseAll: func() (tea.Model, tea.Cmd) {
			for _, node := range m.nodes {
				if node.Level > 0 {
					node.Expanded = false
				}
			}
			m.updateVisibility()
			return m, nil
		},
		FocusSubtree: func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				task := v[m.cursor].Task
				if task.Type.CanHaveChildren() {
					m.focusedTask = task
					m.cursor = 0
					m.buildTree()
				}
			}
			return m, nil
		},
		ToggleInlinePreview: func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		},
	})...)

	// Status group: , prefix
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
				_, cmd := m.handleStatusTransition(s, visible())
				return m, cmd
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

	// Priority group: ! prefix
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

	// Due date group: d prefix
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

	// Snooze group: s prefix
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

	// Move group: m prefix
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				_, cmd := m.sendTasksToSection(section, visible())
				return m, cmd
			}
		},
		MoveToTop:    func() (tea.Model, tea.Cmd) { return m.moveTaskToTop(visible()) },
		MoveToBottom: func() (tea.Model, tea.Cmd) { return m.moveTaskToBottom(visible()) },
	})
	if len(moveEntries) > 0 && moveEntries[0].PrefixNode != nil {
		moveEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, moveEntries...)

	// Linked task group: c prefix
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
		UnlinkTask: func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				handled, cmd := handleLinkedTaskPrefixKey("x", v[m.cursor].Task, m.project, true, true)
				if handled {
					return m, cmd
				}
			}
			return m, nil
		},
	})
	if len(linkedEntries) > 0 && linkedEntries[0].PrefixNode != nil {
		linkedEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, linkedEntries...)

	// Task prefix group: t prefix
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
	})
	if len(taskPrefixEntries) > 0 && taskPrefixEntries[0].PrefixNode != nil {
		taskPrefixEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, taskPrefixEntries...)

	// Stage group: . prefix — map stage name back to a prefix key using StageForPrefixKey inverse.
	// We use handleStagePrefix with the key that would resolve to the given stage.
	stageEntries := keymap.StageGroup(keymap.StageDeps{
		Stages: stages,
		ChangeStage: func(stage string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				// Find the key that maps to this stage and delegate to the shared handler.
				for _, k := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
					if s := StageForPrefixKey(k, stages); s != nil && *s == stage {
						cmd, _ := handleStagePrefix(k, m, stages)
						return m, cmd
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

	// Blocker group: b prefix
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
		blockerEntries[0].PrefixNode.Gate = hasTask
	}
	entries = append(entries, blockerEntries...)

	// Filter group: F prefix
	entries = append(entries, keymap.FilterGroup(keymap.FilterDeps{
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
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("F.", key) }
		},
		StageFilterNames: stages,
	})...)

	// Type change: < > H L
	entries = append(entries,
		keymap.Bind("<", "Promote task type", "Task Operations", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleTypeChangeKey("<", m)
			return m, cmd
		}),
		keymap.Bind(">", "Demote task type", "Task Operations", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleTypeChangeKey(">", m)
			return m, cmd
		}),
		keymap.Bind("H", "Cycle type left", "Task Operations", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleTypeChangeKey("H", m)
			return m, cmd
		}),
		keymap.Bind("L", "Cycle type right", "Task Operations", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleTypeChangeKey("L", m)
			return m, cmd
		}),
	)

	// Selection/expansion keys
	entries = append(entries,
		keymap.Bind("space", "Expand/collapse or toggle selection", "Navigation", func() (tea.Model, tea.Cmd) {
			v := visible()
			if m.multiSelectMode {
				if CursorInBounds(m.cursor, len(v)) {
					m.toggleSelection(v[m.cursor].Task)
				}
				return m, nil
			}
			if CursorInBounds(m.cursor, len(v)) {
				node := v[m.cursor]
				if len(node.Task.Children) > 0 {
					node.Expanded = !node.Expanded
					m.updateVisibility()
				}
			}
			return m, nil
		}),
		keymap.Bind("tab", "Cycle sidebar state", "Navigation", func() (tea.Model, tea.Cmd) {
			v := visible()
			var task *models.Task
			if CursorInBounds(m.cursor, len(v)) {
				task = v[m.cursor].Task
			}
			m.sidebar.CycleState(task)
			if m.sidebar.IsFocused() && task != nil {
				sections := sidebarGetAvailableSections(m.project, task)
				if len(sections) > 0 {
					m.sidebar.SetDependencySection(sections[0])
				}
			}
			return m, nil
		}),
		keymap.Bind("v", "Enter multi-select mode", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.clearSelection()
			} else {
				m.multiSelectMode = true
				v := visible()
				if CursorInBounds(m.cursor, len(v)) {
					m.toggleSelection(v[m.cursor].Task)
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
		keymap.Bind("A", "Deselect all or archive/all-tasks", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAllVisible()
				return m, nil
			}
			if m.viewMode == ViewModeCompleted {
				completedCount := m.project.CountCompletedTasks()
				if completedCount > 0 {
					m.confirm.Show("archive-tasks",
						fmt.Sprintf("Archive %d completed task(s) to completed.md?", completedCount))
				}
				return m, nil
			}
			return m, func() tea.Msg { return ShowAllTasksMsg{} }
		}),
		keymap.Bind("esc", "Exit multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.clearSelection()
			return m, nil
		}, keymap.WithGate(func() bool { return m.multiSelectMode })),
	)

	// Task movement keys
	entries = append(entries,
		keymap.Bind("K", "Move task up", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("K", visible())
			return m, cmd
		}),
		keymap.Bind("J", "Move task down", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("J", visible())
			return m, cmd
		}),
		keymap.Bind("right", "Indent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("right", visible())
			return m, cmd
		}, keymap.AsHidden()),
		keymap.Bind("left", "Outdent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("left", visible())
			return m, cmd
		}, keymap.AsHidden()),
	)

	// Action keys
	entries = append(entries,
		keymap.Bind("enter", "View task details", "Navigation", func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				task := v[m.cursor].Task
				project := m.project
				m.clearSelection()
				return m, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: project} }
			}
			return m, nil
		}),
		keymap.Bind("o", "View task details", "Task Operations", func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				task := v[m.cursor].Task
				project := m.project
				m.clearSelection()
				return m, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: project} }
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("e", "Edit task", "Task Operations", func() (tea.Model, tea.Cmd) {
			v := visible()
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m, func() tea.Msg {
					return ShowBulkEditMsg{Tasks: m.getSelectedTasks(), Project: m.project}
				}
			}
			if CursorInBounds(m.cursor, len(v)) {
				return m, func() tea.Msg { return EditTaskMsg{Task: v[m.cursor].Task} }
			}
			return m, nil
		}),
		keymap.Bind("E", "Quick edit (inline)", "Task Operations", func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				m.enterQuickEditMode(v[m.cursor].Task, false)
			}
			return m, nil
		}),
		keymap.Bind("D", "Delete task", "Task Operations", func() (tea.Model, tea.Cmd) {
			handleDeleteConfirmKey(m)
			return m, nil
		}),
		keymap.Bind("*", "Toggle active flag", "Task Operations", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleToggleActiveKey(m) //nolint:errcheck
			return m, cmd
		}),
		keymap.Bind("ctrl+;", "Toggle focus mode", "Navigation", func() (tea.Model, tea.Cmd) {
			v := visible()
			if CursorInBounds(m.cursor, len(v)) {
				task := v[m.cursor].Task
				if m.focusedTask != nil && m.focusedTask.ID == task.ID {
					m.focusedTask = nil
				} else if task.Type.CanHaveChildren() {
					m.focusedTask = task
					m.cursor = 0
				}
				m.buildTree()
			}
			return m, nil
		}),
		keymap.Bind("f", "Focus view (project)", "Views", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectFocusViewMsg{Project: m.project} }
		}),
		keymap.Bind("C", "Filter by context", "Filters", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowContextPickerMsg{} }
		}),
		keymap.Bind("T", "Filter by tag", "Filters", func() (tea.Model, tea.Cmd) {
			cmd, _ := handleTagKey(m, ViewProjectTree, false, m.filter.Tags)
			return m, cmd
		}),
		keymap.Bind("ctrl+a", "Archive/unarchive project", "Project", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, nil
			}
			if m.project.Sandbox {
				return m, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
				}
			}
			if m.project.Archived {
				m.confirm.Show("unarchive", "Are you sure you want to unarchive this project?")
			} else {
				m.confirm.Show("archive", "Are you sure you want to archive this project?")
			}
			return m, nil
		}),
		keymap.Bind("ctrl+e", "Export project to Markdown", "Project", func() (tea.Model, tea.Cmd) {
			project := m.getProject()
			return m, func() tea.Msg { return ExportProjectMsg{Project: project} }
		}),
		keymap.Bind("ctrl+f", "Filter summary", "Filters", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFilterMsg{ProjectTree: m} }
		}),
		keymap.Bind("ctrl+t", "Tag manager", "Filters", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowTagManagerMsg{ProjectTree: m} }
		}),
		keymap.Bind("ctrl+s", "Section manager", "Filters", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowSectionManagerMsg{ProjectTree: m} }
		}),
		keymap.Bind("ctrl+v", "Create task from clipboard", "Task Operations", func() (tea.Model, tea.Cmd) {
			return m, m.pasteFromClipboard()
		}),
		keymap.Bind("shift+left", "Previous custom section tab", "Navigation", func() (tea.Model, tea.Cmd) {
			tabs := m.customSectionTabs()
			if tabs != nil {
				cycleCustomSectionTab("shift+left", &m.customSectionTab, len(tabs), func() {
					m.updateVisibility()
					m.clampCursor()
				})
			}
			return m, nil
		}),
		keymap.Bind("shift+right", "Next custom section tab", "Navigation", func() (tea.Model, tea.Cmd) {
			tabs := m.customSectionTabs()
			if tabs != nil {
				cycleCustomSectionTab("shift+right", &m.customSectionTab, len(tabs), func() {
					m.updateVisibility()
					m.clampCursor()
				})
			}
			return m, nil
		}),
		keymap.Bind("/", "Search tasks", "Filters", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
	)

	// Sort keys: 1-5, 0
	for _, k := range []string{"0", "1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Sort by "+key, "Sort", func() (tea.Model, tea.Cmd) {
				m.handleSortKeys(key)
				return m, nil
			}, keymap.AsHidden()),
		)
	}

	return &keymap.KeyMap{
		ViewName: "Project Tree",
		Entries:  entries,
	}
}
