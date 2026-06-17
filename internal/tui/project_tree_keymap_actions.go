package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (b *projectTreeKeymapBuilder) typeChangeEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
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
	}
}

func (b *projectTreeKeymapBuilder) selectionEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("space", "Toggle selection (multi-select) / toggle peek (use zo for fold)", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				task := b.cursorTask()
				if task != nil {
					m.toggleSelection(task)
				}
			}

			return m, nil
		}),
		keymap.Bind("tab", "Cycle sidebar state", "Navigation", func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
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
				return m, nil
			}

			m.multiSelectMode = true
			task := b.cursorTask()
			if task != nil {
				m.toggleSelection(task)
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
	}
}

func (b *projectTreeKeymapBuilder) movementEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("K", "Move task up", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("K", b.visible())
			return m, cmd
		}),
		keymap.Bind("J", "Move task down", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("J", b.visible())
			return m, cmd
		}),
		keymap.Bind("right", "Indent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("right", b.visible())
			return m, cmd
		}, keymap.AsHidden()),
		keymap.Bind("left", "Outdent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleMovementKeys("left", b.visible())
			return m, cmd
		}, keymap.AsHidden()),
	}
}

func (b *projectTreeKeymapBuilder) commonTaskActionEntries() []keymap.Entry {
	m := b.model

	return keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil {
				return m, nil
			}

			m.clearSelection()
			return m, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: m.project} }
		},
		SnoozeDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := resolveTaskAndProject(m, m.getProject())
			if task == nil {
				return m, nil
			}

			return m, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
		MarkDone: func() (tea.Model, tea.Cmd) {
			_, cmd := m.handleStatusTransition(models.StatusDone, b.visible())
			return m, cmd
		},
		EditTask: func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && len(m.selectedTasks) > 0 {
				return m, func() tea.Msg {
					return ShowBulkEditMsg{Tasks: m.getSelectedTasks(), Project: m.project}
				}
			}

			task := b.cursorTask()
			if task == nil {
				return m, nil
			}

			return m, func() tea.Msg { return EditTaskMsg{Task: task} }
		},
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(ViewProjectTree, "Project Tree", GetKeybindingRegistry().GetForView(ViewProjectTree))
		},
		ToggleActive: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleToggleActiveKey(m) //nolint:errcheck
			return m, cmd
		},
		EditScore: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleResolvedTaskKey("S", m, m.getProject(), ViewProjectTree)
			return m, cmd
		},
		OpenURL: func() (tea.Model, tea.Cmd) {
			task, _ := resolveTaskAndProject(m, m.getProject())
			if task == nil || task.URL == "" {
				return m, nil
			}

			return m, openBrowserCmd(task.URL)
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := resolveTaskAndProject(m, m.getProject())
			if task == nil || project == nil {
				return m, nil
			}

			return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		QuickNote: func() (tea.Model, tea.Cmd) {
			cmd, _ := handleResolvedTaskKey("ctrl+.", m, m.getProject(), ViewProjectTree)
			return m, cmd
		},
	})
}

func (b *projectTreeKeymapBuilder) actionEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("enter", "View task details", "Navigation", func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil {
				return m, nil
			}

			m.clearSelection()
			return m, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: m.project} }
		}),
		keymap.Bind("E", "Quick edit (inline)", "Task Operations", func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task != nil {
				m.enterQuickEditMode(task, false)
			}

			return m, nil
		}),
		keymap.Bind("ctrl+;", "Toggle focus mode", "Navigation", func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task != nil {
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
		keymap.Bind("alt+f", "Focus view (project)", "Views", func() (tea.Model, tea.Cmd) {
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

			return m, projectArchiveRequestCmd(m.project, !m.project.Archived)
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
		keymap.Bind("shift+left", "Previous pinned tag tab", "Navigation", func() (tea.Model, tea.Cmd) {
			tabs := m.pinnedTagTabs()
			if tabs != nil {
				cyclePinnedTagTab("shift+left", &m.pinnedTagTab, len(tabs), func() {
					m.updateVisibility()
					m.clampCursor()
				})
			}
			return m, nil
		}),
		keymap.Bind("shift+right", "Next pinned tag tab", "Navigation", func() (tea.Model, tea.Cmd) {
			tabs := m.pinnedTagTabs()
			if tabs != nil {
				cyclePinnedTagTab("shift+right", &m.pinnedTagTab, len(tabs), func() {
					m.updateVisibility()
					m.clampCursor()
				})
			}
			return m, nil
		}),
		keymap.Bind("/", "Search tasks", "Filters", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
	}
}

func (b *projectTreeKeymapBuilder) sortEntries() []keymap.Entry {
	m := b.model
	entries := make([]keymap.Entry, 0, 6)

	for _, value := range []string{"0", "1", "2", "3", "4", "5"} {
		key := value
		entries = append(entries, keymap.Bind(key, "Sort by "+key, "Sort", func() (tea.Model, tea.Cmd) {
			m.handleSortKeys(key)
			return m, nil
		}, keymap.AsHidden()))
	}

	return entries
}
