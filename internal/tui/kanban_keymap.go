package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for KanbanModel.
func (m *KanbanModel) buildKeyMap() *keymap.KeyMap {
	hasCard := func() bool { return m.getCursorCard() != nil }

	var entries []keymap.Entry

	// --- Column navigation (not using ListNavigationGroup — kanban is column-based) ---
	entries = append(entries,
		keymap.Bind("j", "Move cursor down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorDown()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorDown()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorUp()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorUp()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("h", "Focus previous column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveFocusPrevCol()
			return m, nil
		}),
		keymap.Bind("l", "Focus next column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveFocusNextCol()
			return m, nil
		}),
		keymap.Bind("ctrl+u", "Page up within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.pageUp()
			return m, nil
		}),
		keymap.Bind("ctrl+d", "Page down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.pageDown()
			return m, nil
		}),
		keymap.Bind("G", "Jump to bottom of column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.jumpToBottom()
			return m, nil
		}),
		keymap.Bind("enter", "Open task detail", "Navigation", func() (tea.Model, tea.Cmd) {
			if cmd := m.openTaskDetailCmd(); cmd != nil {
				return m, cmd
			}
			return m, nil
		}),
	)

	// --- H/L: move task to adjacent column; J/K: reorder within column ---
	entries = append(entries,
		keymap.Bind("H", "Move task to left column", "Actions", func() (tea.Model, tea.Cmd) {
			return m.moveTaskLeft()
		}),
		keymap.Bind("L", "Move task to right column", "Actions", func() (tea.Model, tea.Cmd) {
			return m.moveTaskRight()
		}),
		keymap.Bind("J", "Reorder card down", "Actions", func() (tea.Model, tea.Cmd) {
			m.reorderCardDown()
			return m, nil
		}),
		keymap.Bind("K", "Reorder card up", "Actions", func() (tea.Model, tea.Cmd) {
			m.reorderCardUp()
			return m, nil
		}),
	)

	// --- Goto group: gg (top), gp (project) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return KanbanSelectMsg{Task: task, Project: project} }
		},
	})...)

	// --- View mode keys ---
	entries = append(entries,
		keymap.Bind("tab", "Toggle bottom detail pane", "View", func() (tea.Model, tea.Cmd) {
			m.detailPaneVisible = !m.detailPaneVisible
			return m, nil
		}),
		keymap.Bind("`", "Open mode picker", "View", func() (tea.Model, tea.Cmd) {
			m.showModePicker = true
			return m, nil
		}),
		keymap.Bind("~", "Cycle axis mode back", "View", func() (tea.Model, tea.Cmd) {
			m.cycleAxisModeBack()
			return m, nil
		}),
		keymap.Bind("@", "Open axis mode manager", "View", func() (tea.Model, tea.Cmd) {
			m.showAxisManager = true
			m.axisCursor = 0
			return m, nil
		}),
	)

	// --- Multi-select ---
	entries = append(entries,
		keymap.Bind("V", "Enter multi-select mode", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.ToggleMode(&m.statusMsg, "Multi-select mode — space: select, H/L: bulk move, esc: exit")
			return m, nil
		}),
		keymap.Bind("space", "Toggle card selection (multi-select) / toggle peek", "Multi-select", func() (tea.Model, tea.Cmd) {
			if card := m.getCursorCard(); card != nil {
				m.ToggleAndUpdateStatus(card.Task.ID, &m.statusMsg,
					"Multi-select: navigate and press space to select",
					"%d task(s) selected")
			}
			return m, nil
		}),
		keymap.Bind("esc", "Exit multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.ExitMultiSelect()
			m.statusMsg = ""
			return m, nil
		}, keymap.WithGate(func() bool { return m.multiSelectMode })),
	)

	// --- Direct action keys ---
	entries = append(entries,
		keymap.Bind("a", "Add task (quick capture)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{
					Project:     m.project,
					AllProjects: m.runtime.ActiveProjects(),
				}
			}
		}),
		keymap.Bind("r", "Refresh", "Actions", func() (tea.Model, tea.Cmd) {
			m.reloadData()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// --- Task action groups (Status, Priority, DueDate, Snooze, Move, Linked, TaskPrefix, Blocker, CommonTaskAction) ---
	entries = append(entries, buildStandardTaskActionEntries(TaskKeymapDeps{
		View:    m,
		HasItem: hasCard,
		SelectedTaskRefs: func() []TaskProjectRef {
			return m.selectedTaskProjectRefs()
		},
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
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
		DeleteTask: func() (tea.Model, tea.Cmd) {
			return m.applySelectedBulkDelete()
		},
		StateChange: func(state string) keymap.Action {
			s := models.WorkflowState(state)
			return func() (tea.Model, tea.Cmd) {
				return m.applySelectedBulkState(s)
			}
		},
		PriorityChange: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.applySelectedBulkPriority(key)
			}
		},
		DueDateSet: func(days int) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.applySelectedBulkDueDate(days, false)
			}
		},
		DueDateClear: func() (tea.Model, tea.Cmd) {
			return m.applySelectedBulkDueDate(0, true)
		},
		SnoozeDate: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.applySelectedBulkSnooze(key)
			}
		},
		SectionMove: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return m.applySelectedBulkSectionMove(section)
			}
		},
		MarkDone: func() (tea.Model, tea.Cmd) {
			return m.applySelectedBulkState(models.StateDone)
		},
		TogglePin: func() (tea.Model, tea.Cmd) {
			return m.applySelectedBulkTogglePin()
		},
		ToggleDim: func() (tea.Model, tea.Cmd) {
			return m.applySelectedBulkToggleDim()
		},
	})...)

	// --- Filter group: "F" prefix, no gate ---
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("#") },
		TogglePriority:    func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("p") },
		ToggleHideDimmed:  func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("h") },
		ToggleGitHubIssue: func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("i") },
		ToggleParked:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("k") },
		ClearFilters:      func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Ft", key) }
		},
		StateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fd", key) }
		},
		OpenAttrPicker: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: m.viewID()} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
	})
	entries = append(entries, filterEntries...)

	// --- Perspective lens (cross-project mode only): "P" prefix ---
	if m.crossProject {
		entries = append(entries, BuildPerspectivePrefixEntry(PerspectivePrefixDeps{
			SelectByID: func(perspID int) (tea.Model, tea.Cmd) {
				m.activePerspective = perspID
				m.rebuildCols()
				m.clampCursor()
				name := PerspectiveLensName(perspID)
				if name != "" {
					m.setStatus(fmt.Sprintf("Perspective: %s", name))
				}
				return m, nil
			},
			CycleNext: func() (tea.Model, tea.Cmd) {
				m.activePerspective = CyclePerspectiveLens(m.activePerspective, 1)
				m.rebuildCols()
				m.clampCursor()
				m.setStatus(fmt.Sprintf("Perspective: %s", PerspectiveLensName(m.activePerspective)))
				return m, nil
			},
			CyclePrev: func() (tea.Model, tea.Cmd) {
				m.activePerspective = CyclePerspectiveLens(m.activePerspective, -1)
				m.rebuildCols()
				m.clampCursor()
				m.setStatus(fmt.Sprintf("Perspective: %s", PerspectiveLensName(m.activePerspective)))
				return m, nil
			},
		}))
	}

	viewName := "Kanban"
	if m.crossProject {
		viewName = "Cross-Project Kanban"
	}
	return &keymap.KeyMap{
		ViewName: viewName,
		Entries:  entries,
	}
}

// HandledKeys implements KeySyncView.
func (m *KanbanModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
