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

	// The retired Ctrl+P project drilldown is gone: project scope is entered
	// exclusively through Ctrl+X (the scope picker) and the project-header scope
	// actions, so CommonPrimaryViewGroup omits the binding on both the
	// single-project and cross-project outliners.
	entries := keymap.CommonPrimaryViewGroup(keymap.CommonPrimaryViewDeps{
		ProjectBlockers: func() (tea.Model, tea.Cmd) {
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
			}
			return m, func() tea.Msg { return ShowBlockersMsg{Project: project} }
		},
		PriorityView: func() (tea.Model, tea.Cmd) {
			if m.activeOutlinerMode() == ModePriorityLens {
				mode, ok := OutlinerModeForPerspective(m.activePerspective)
				if !ok {
					return m, nil
				}
				return m.selectOutlinerMode(mode.ID)
			}
			return m.selectOutlinerMode(ModePriorityLens)
		},
		InboxProcessor: func() (tea.Model, tea.Cmd) {
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
			}
			return m, func() tea.Msg { return ShowInboxProcessorMsg{Project: project} }
		},
		QuickCapture: func() (tea.Model, tea.Cmd) {
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
			}
			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{Project: project, AllProjects: m.runtime.ActiveProjects()}
			}
		},
		BulkCapture: func() (tea.Model, tea.Cmd) {
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
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
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
			}
			return m, func() tea.Msg { return ShowNotesMsg{Project: project} }
		},
		CreateNote: func() (tea.Model, tea.Cmd) {
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
			}
			return m, func() tea.Msg { return ShowNoteCreatorMsg{Project: project} }
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
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
			}
			recent := storage.GetRecentTasks(project.Tasks, 1)
			if len(recent) == 0 {
				m.setStatus("No recent tasks")
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
			project := m.contextProject()
			if project == nil {
				m.setNoProjectStatus()
				return m, nil
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
			m.resetViewScrollOffset()
			return m.selectOutlinerMode(ModeSnoozed)
		}),
	})}
}

// modeEntries binds the mode surface: P opens the searchable, categorized mode
// picker (replacing the retired P0-P9 numeric prefix), and Tab/Shift+Tab cycle
// the five-item Work rotation Today → Next → Backlog → Active → All.
//
// R carries a single, non-conflicting behavior that depends on the active mode:
// in Daily Review mode it marks or resets the persisted review timestamp;
// otherwise it reloads the visible projects from disk. The earlier duplicate
// R=reload binding in the shared primary-view group was retired so this one
// dispatch owns the key.
func (b *outlinerKeymapBuilder) modeEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("P", "Switch mode", "Modes", func() (tea.Model, tea.Cmd) {
			return m.openModePicker()
		}),
		keymap.Bind("tab", "Next work mode", "Modes", func() (tea.Model, tea.Cmd) {
			return m.cycleWorkMode(1)
		}),
		keymap.Bind("shift+tab", "Previous work mode", "Modes", func() (tea.Model, tea.Cmd) {
			return m.cycleWorkMode(-1)
		}),
		keymap.Bind("R", "Reload / complete daily review", "Modes", func() (tea.Model, tea.Cmd) {
			if m.reviewMode == reviewDaily {
				return m.handleReviewCompletionToggle()
			}
			return m, m.reloadFromDisk()
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
