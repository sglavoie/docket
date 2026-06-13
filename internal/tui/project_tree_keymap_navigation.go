package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (b *projectTreeKeymapBuilder) primaryViewEntries() []keymap.Entry {
	m := b.model

	return keymap.CommonPrimaryViewGroup(keymap.CommonPrimaryViewDeps{
		OpenProjectTree: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectTreeMsg{Project: m.getProject()} }
		},
		OpenProjectCyclePicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowProjectCyclePickerMsg{} }
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
			suggestedTag := m.singleActiveTag()
			suggestedSection := m.singleActiveSection()

			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{
					Project:          project,
					AllProjects:      cfg.Projects,
					SuggestedTag:     suggestedTag,
					SuggestedSection: suggestedSection,
				}
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
	})
}

func (b *projectTreeKeymapBuilder) navigationEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := b.visible()
			if m.cursor < len(visible)-1 {
				m.cursor++
			}

			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := b.visible()
			if m.cursor < len(visible)-1 {
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
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}

			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("G", "Jump to last", "Navigation", func() (tea.Model, tea.Cmd) {
			visible := b.visible()
			old := m.cursor
			if len(visible) > 0 {
				m.cursor = len(visible) - 1
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
			return m, m.SectionGoToNext(rootNodeIndices(b.visible()))
		}),
		keymap.Bind("N", "Previous root task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.SectionGoToPrev(rootNodeIndices(b.visible()))
		}),
	}
}

func (b *projectTreeKeymapBuilder) gotoEntries() []keymap.Entry {
	m := b.model
	setViewMode := func(mode ViewMode) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			m.viewMode = mode
			m.updateVisibility()
			m.pruneInvisibleSelections()
			m.clampCursor()
			return m, nil
		}
	}

	return keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop:   func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: nil,
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
			keymap.Bind("c", "Go to Completed view", "Views", setViewMode(ViewModeCompleted)),
			keymap.Bind("*", "Go to All view", "Views", setViewMode(ViewModeAll)),
			keymap.Bind("r", "Go to Review view", "Views", setViewMode(ViewModeReview)),
			keymap.Bind("S", "Go to Standby view", "Views", setViewMode(ViewModeStandby)),
			keymap.Bind("o", "Jump to context node's section", "Navigation", func() (tea.Model, tea.Cmd) {
				node := b.currentNode()
				if node == nil || !node.IsContextNode {
					return m, nil
				}

				task := node.Task
				m.selectViewForTask(task)
				m.buildTree()
				return m, m.NavigateToTask(task)
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
					for _, node := range b.visible() {
						delete(m.selectedTasks, node.Task.ID)
					}
				}

				return m, nil
			}),
			keymap.Bind("h", "View task activity log", "Task Operations", func() (tea.Model, tea.Cmd) {
				node := b.currentNode()
				if node == nil {
					return m, nil
				}

				return m, func() tea.Msg {
					return OpenTaskActivityMsg{Task: node.Task, Project: m.project}
				}
			}),
			keymap.Bind("l", "Project recent activity", "Navigation", func() (tea.Model, tea.Cmd) {
				return m, func() tea.Msg { return ShowProjectRecentActivityMsg{Project: m.project} }
			}),
			keymap.Bind("k", "Open kanban view", "Navigation", func() (tea.Model, tea.Cmd) {
				return m, func() tea.Msg { return ShowKanbanMsg{Project: m.project} }
			}),
		},
	})
}

func (b *projectTreeKeymapBuilder) foldEntries() []keymap.Entry {
	m := b.model

	return keymap.FoldGroup(keymap.FoldDeps{
		ToggleFold: func() (tea.Model, tea.Cmd) {
			node := b.currentNode()
			if node != nil && len(node.Task.Children) > 0 {
				node.Expanded = !node.Expanded
				m.updateVisibility()
			}

			return m, nil
		},
		FocusSubtree: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task != nil && task.Type.CanHaveChildren() {
				m.focusedTask = task
				m.cursor = 0
				m.buildTree()
			}

			return m, nil
		},
		ToggleInlinePreview: func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		},
		CollapseToDepth1: func() (tea.Model, tea.Cmd) {
			for _, node := range m.nodes {
				node.Expanded = false
			}
			m.updateVisibility()
			return m, nil
		},
		CollapseToDepth2: func() (tea.Model, tea.Cmd) {
			for _, node := range m.nodes {
				node.Expanded = node.Level < 1
			}
			m.updateVisibility()
			return m, nil
		},
		CollapseToDepth3: func() (tea.Model, tea.Cmd) {
			for _, node := range m.nodes {
				node.Expanded = node.Level < 2
			}
			m.updateVisibility()
			return m, nil
		},
	})
}
