package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for KanbanModel.
func (m *KanbanModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasCard := func() bool { return m.getCursorCard() != nil }

	var entries []keymap.Entry

	// --- Column navigation (not using ListNavigationGroup — kanban is column-based) ---
	entries = append(entries,
		keymap.Bind("j", "Move cursor down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorDown()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up within column", "Navigation", func() (tea.Model, tea.Cmd) {
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

	// --- f prefix: fR, fM (no-op stubs in kanban) ---
	entries = append(entries, keymap.Prefix("f", "View", "View", []keymap.Entry{
		keymap.Bind("R", "Expand all sections", "View", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("M", "Collapse all sections", "View", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	}))

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
		keymap.Bind("space", "Toggle multi-select on card", "Multi-select", func() (tea.Model, tea.Cmd) {
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
					AllProjects: m.config.GetActiveProjects(),
				}
			}
		}),
		keymap.Bind("M", "Move to custom section (picker)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.mode == KanbanModeCustomSections {
				card := m.getCursorCard()
				if card != nil {
					projects := m.loadedProjects
					return m, func() tea.Msg {
						return ShowCustomSectionPickerMsg{Task: card.Task, Project: card.Project, Projects: projects}
					}
				}
			}
			return m, nil
		}),
		keymap.Bind("r", "Refresh", "Actions", func() (tea.Model, tea.Cmd) {
			m.reloadData()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// --- Task action groups (Status, Priority, DueDate, Snooze, Move, Linked, TaskPrefix, Stage, Blocker, CommonTaskAction) ---
	entries = append(entries, buildStandardTaskActionEntries(TaskKeymapDeps{
		View:    m,
		HasItem: hasCard,
		Stages:  stages,
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
		DueDateSet: func(days int) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
					var count int
					for _, col := range m.columns {
						for i := range col.Tasks {
							card := &col.Tasks[i]
							if !m.IsSelected(card.Task.ID) {
								continue
							}
							pf := workflowPersist(func() WorkflowSaveOptions {
								return BuildWorkflowOpts(m.getPersister(), card.Project, m.config, m.configManager, nil, "", "", nil)
							})
							d := dueDate
							pf(card.Project, card.Task.ID, func(t *models.Task) {
								t.DueDate = &d
								t.HasTime = false
							})
							count++
						}
					}
					m.ExitMultiSelect()
					m.setStatus(fmt.Sprintf("Due date set on %d tasks", count))
					m.reloadData()
					m.clampCursor()
					return m, nil
				}
				return taskOpsSetQuickDueDate(m, days)
			}
		},
	})...)

	// --- Filter group: "F" prefix, no gate ---
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
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
			return m, func() tea.Msg { return ShowAttrFilterPickerMsg{Source: m.viewID()} }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("F.", key) }
		},
		StageFilterNames: stages,
	})
	entries = append(entries, filterEntries...)

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
