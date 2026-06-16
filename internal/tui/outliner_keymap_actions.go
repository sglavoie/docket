package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func (b *outlinerKeymapBuilder) typeChangeEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("<", "Promote task type", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("<")
			return model, cmd
		}),
		keymap.Bind(">", "Demote task type", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey(">")
			return model, cmd
		}),
		keymap.Bind("H", "Cycle type left", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("H")
			return model, cmd
		}),
		keymap.Bind("L", "Cycle type right", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("L")
			return model, cmd
		}),
	}
}

func (b *outlinerKeymapBuilder) perspectiveCycleEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("h", "Cycle perspective left", "Perspectives", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m.crossHandlePerspectiveCycle("h")
			}
			perspCount := len(PerspectivePresets())
			m.activePerspective = (m.activePerspective + perspCount - 1) % perspCount
			m.rebuildRows()
			return m, nil
		}),
		keymap.Bind("l", "Cycle perspective right", "Perspectives", func() (tea.Model, tea.Cmd) {
			if m.project == nil {
				return m.crossHandlePerspectiveCycle("l")
			}
			perspCount := len(PerspectivePresets())
			m.activePerspective = (m.activePerspective + 1) % perspCount
			m.rebuildRows()
			return m, nil
		}),
	}
}

func (b *outlinerKeymapBuilder) sectionNavigationEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("n", "Jump to next section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.jumpToNextSection(1)
		}),
		keymap.Bind("N", "Jump to previous section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.jumpToNextSection(-1)
		}),
	}
}

func (b *outlinerKeymapBuilder) actionEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("enter", "Zoom into parent / open leaf", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleNavigationKey("enter")
			return model, cmd
		}),
		keymap.Bind("backspace", "Zoom out one level", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleNavigationKey("backspace")
			return model, cmd
		}),
		keymap.Bind("tab", "Cycle sidebar state", "Sidebar", func() (tea.Model, tea.Cmd) {
			task := m.SidebarSelectedTask()
			m.sidebar.CycleState(task)
			if m.sidebar.IsFocused() && task != nil {
				if proj := m.SidebarProject(); proj != nil {
					sections := sidebarGetAvailableSections(proj, task)
					if len(sections) > 0 {
						m.sidebar.SetDependencySection(sections[0])
					}
				}
			}
			return m, nil
		}),
	}
}

func (b *outlinerKeymapBuilder) movementEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("J", "Move task down", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("J", b.onHeader())
			return model, cmd
		}),
		keymap.Bind("K", "Move task up", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("K", b.onHeader())
			return model, cmd
		}),
		keymap.Bind("right", "Indent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("right", b.onHeader())
			return model, cmd
		}, keymap.AsHidden()),
		keymap.Bind("left", "Outdent task", "Task Movement", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskMutationKey("left", b.onHeader())
			return model, cmd
		}, keymap.AsHidden()),
	}
}

func (b *outlinerKeymapBuilder) commonTaskActionEntries() []keymap.Entry {
	m := b.model
	helpView := ViewOutliner
	helpTitle := "Outliner"
	if m.project == nil {
		helpView = ViewCrossProjectOutliner
		helpTitle = "Cross-Project Outliner"
	}

	return keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("o", b.onHeader())
			return model, cmd
		},
		SnoozeDatePicker: func() (tea.Model, tea.Cmd) {
			if refs := m.getSelectedTaskProjectRefs(); len(refs) > 0 {
				return m, func() tea.Msg { return SetSnoozeDateMsg{TaskRefs: refs} }
			}
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
		MarkDone: func() (tea.Model, tea.Cmd) {
			return m.handleStatusPrefix("d", b.cursorTask())
		},
		EditTask: func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("e", b.onHeader())
			return model, cmd
		},
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(helpView, helpTitle, GetKeybindingRegistry().GetForView(helpView))
		},
		ToggleActive: func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("*")
			return model, cmd
		},
		EditScore: func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg {
				return ShowScoreEditorMsg{Task: b.cursorTask(), Project: m.project}
			}
		},
		OpenURL: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil || task.URL == "" {
				return m, nil
			}
			return m, openBrowserCmd(task.URL)
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil || m.project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: m.project} }
		},
		QuickNote: func() (tea.Model, tea.Cmd) {
			task := b.cursorTask()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: m.project} }
		},
	})
}

func (b *outlinerKeymapBuilder) taskViewEntries() []keymap.Entry {
	m := b.model

	return []keymap.Entry{
		keymap.Bind("E", "Quick edit (inline)", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("E", b.onHeader())
			return model, cmd
		}),
		keymap.Bind("ctrl+f", "Filter summary", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleTaskViewKey("ctrl+f", b.onHeader())
			return model, cmd
		}),
	}
}

func (b *outlinerKeymapBuilder) utilityEntries() []keymap.Entry {
	m := b.model
	tagManagerDesc := "Tag manager"
	if m.project == nil {
		tagManagerDesc = "Global tag manager"
	}

	return []keymap.Entry{
		keymap.Bind("v", "Toggle multi-select mode", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection (multi-select) / hold to peek", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				task := b.cursorTask()
				if task != nil {
					m.ToggleSelection(task.ID)
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
		keymap.Bind("A", "Deselect all / archive / all-tasks", "Views", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAllVisible()
				return m, nil
			}
			model, cmd, _ := m.handleUtilityKey("A")
			return model, cmd
		}),
		keymap.Bind("T", "Tag filter selector", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("T")
			return model, cmd
		}),
		keymap.Bind("C", "Context filter", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("C")
			return model, cmd
		}),
		keymap.Bind("f", "Focus view (project)", "Views", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("f")
			return model, cmd
		}),
		keymap.Bind("shift+left", "Previous pinned tag tab", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("shift+left")
			return model, cmd
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next pinned tag tab", "Navigation", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("shift+right")
			return model, cmd
		}, keymap.AsHidden()),
		keymap.Bind("ctrl+a", "Archive/unarchive project", "Project", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+a")
			return model, cmd
		}),
		keymap.Bind("ctrl+t", tagManagerDesc, "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+t")
			return model, cmd
		}),
		keymap.Bind("ctrl+s", "Section manager", "Filters", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+s")
			return model, cmd
		}),
		keymap.Bind("ctrl+v", "Create task from clipboard", "Task Operations", func() (tea.Model, tea.Cmd) {
			model, cmd, _ := m.handleUtilityKey("ctrl+v")
			return model, cmd
		}),
		keymap.Bind("ctrl+e", "Export project to Markdown", "Project", func() (tea.Model, tea.Cmd) {
			project := m.project
			return m, func() tea.Msg { return ExportProjectMsg{Project: project} }
		}),
		keymap.Bind("/", "Search tasks", "Filters", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
		keymap.Bind("x", "Unlink task (sidebar)", "Sidebar", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	}
}
