package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (b *outlinerKeymapBuilder) navigationEntries() []keymap.Entry {
	m := b.model

	return keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("j", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipNonNavigableRows(prev, cmd)
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("k", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipNonNavigableRows(prev, cmd)
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("G", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipNonNavigableRows(prev, cmd)
			}
			return m, nil
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("ctrl+u", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipNonNavigableRows(prev, cmd)
			}
			return m, nil
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			prev := m.cursor
			handled, cmd := handleListNavigation("ctrl+d", &m.cursor, len(m.rows), m.contentHeight(), &m.cursorFlash)
			if handled {
				return m.crossSkipNonNavigableRows(prev, cmd)
			}
			return m, nil
		},
	})
}

func (b *outlinerKeymapBuilder) primaryViewEntries() []keymap.Entry {
	m := b.model

	entries := keymap.CommonPrimaryViewGroup(keymap.CommonPrimaryViewDeps{
		OpenProjectTree: func() (tea.Model, tea.Cmd) {
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			return m, func() tea.Msg { return ShowProjectTreeMsg{Project: project} }
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
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			return m, func() tea.Msg { return ShowBlockersMsg{Project: project} }
		},
		PriorityView: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				m.toggleCrossPriorityMode()
				return m, nil
			}
			project := m.project
			return m, func() tea.Msg { return ShowPriorityViewMsg{Project: project} }
		},
		AddToPlan: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowFocusBoardMsg{} }
		},
		InboxProcessor: func() (tea.Model, tea.Cmd) {
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			return m, func() tea.Msg { return ShowInboxProcessorMsg{Project: project} }
		},
		QuickCapture: func() (tea.Model, tea.Cmd) {
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{Project: project, AllProjects: m.runtime.ActiveProjects()}
			}
		},
		BulkCapture: func() (tea.Model, tea.Cmd) {
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			return m, func() tea.Msg { return ShowBulkCaptureMsg{Project: project} }
		},
		JumpToTask: func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m, func() tea.Msg {
					return ShowTaskJumperMsg{Projects: m.loadedProjects, CrossProject: true}
				}
			}
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
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
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
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			recent := storage.GetRecentTasks(project.Tasks, 1)
			if len(recent) == 0 {
				return m, nil
			}
			task := recent[0]
			return m, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		MoveReparent: func() (tea.Model, tea.Cmd) {
			source := ViewOutliner
			if m.project == nil {
				source = ViewCrossProjectOutliner
			}
			cmd, _ := handleResolvedTaskKey("ctrl+m", m, m.project, source)
			return m, cmd
		},
	})
	if m.project == nil {
		for i := range entries {
			if entries[i].Binding != nil && entries[i].Binding.Key == "p" {
				entries[i].Binding.Description = "Toggle Priority mode"
			}
		}
	}
	return entries
}

func (b *outlinerKeymapBuilder) gotoEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{keymap.Prefix("g", "Go…", "Navigation", []keymap.Entry{
		keymap.Bind("g", "Go to top", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.JumpToTop()
		}),
		keymap.Bind("h", "View task activity log", "Task Operations", func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				m.setNoTaskStatus()
				return m, nil
			}
			return m, func() tea.Msg { return OpenTaskActivityMsg{Task: task, Project: project} }
		}),
		keymap.Bind("l", "Project recent activity", "Navigation", func() (tea.Model, tea.Cmd) {
			project := m.project
			if project == nil {
				project = m.cursorProject()
				if project == nil {
					m.setNoProjectStatus()
					return m, nil
				}
			}
			return m, func() tea.Msg { return ShowProjectRecentActivityMsg{Project: project} }
		}),
		keymap.Bind("d", "Cycle due date filter", "Filters", func() (tea.Model, tea.Cmd) {
			m.cycleDueDateFilter()
			m.resetViewScrollOffset()
			m.rebuildRows()
			return m, nil
		}),
		keymap.Bind("z", "Jump to Snoozed perspective", "Navigation", func() (tea.Model, tea.Cmd) {
			m.activePerspective = PerspectiveSnoozed
			m.resetViewScrollOffset()
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
	})}
}

func (b *outlinerKeymapBuilder) perspectiveEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Prefix("P", "Switch perspective", "Perspectives", []keymap.Entry{
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
			keymap.Bind("8", "Review perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
				return m.handlePerspectivePrefix("8")
			}),
			keymap.Bind("9", "Snoozed perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
				return m.handlePerspectivePrefix("9")
			}),
			keymap.Bind("0", "Completed perspective", "Perspectives", func() (tea.Model, tea.Cmd) {
				return m.handlePerspectivePrefix("0")
			}),
		}),
	}
}

func (b *outlinerKeymapBuilder) foldEntries() []keymap.Entry {
	m := b.model

	return keymap.FoldGroup(keymap.FoldDeps{
		ToggleFold: func() (tea.Model, tea.Cmd) {
			return m.handleFoldPrefix("a", b.cursorTask())
		},
		ToggleSectionFold: func() (tea.Model, tea.Cmd) {
			m.toggleSectionFoldAtCursor()
			return m, nil
		},
		ToggleAllSectionFolds: func() (tea.Model, tea.Cmd) {
			return m.handleFoldPrefix("S", b.cursorTask())
		},
		FocusSubtree: func() (tea.Model, tea.Cmd) {
			return m.handleFoldPrefix("v", b.cursorTask())
		},
		ToggleInlinePreview: func() (tea.Model, tea.Cmd) {
			return m.handleFoldPrefix("p", b.cursorTask())
		},
		CollapseToDepth1: func() (tea.Model, tea.Cmd) {
			m.collapseToDepth(0)
			return m, nil
		},
		CollapseToDepth2: func() (tea.Model, tea.Cmd) {
			m.collapseToDepth(1)
			return m, nil
		},
		CollapseToDepth3: func() (tea.Model, tea.Cmd) {
			m.collapseToDepth(2)
			return m, nil
		},
	})
}
