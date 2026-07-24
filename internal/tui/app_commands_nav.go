package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) navCommands() []Command {
	return []Command{
		{
			Name:        "outliner root",
			Description: "Reset to the Outliner root (global All)",
			Shortcut:    "ctrl+esc",
			Category:    CategoryNavigation,
			Aliases:     []string{"reset outliner", "global all", "reset to all"},
			Handler: func() tea.Msg {
				return JumpDashboardMsg{}
			},
		},
		{
			Name:        "recent tasks",
			Description: "Quick-switch between recently visited tasks",
			Shortcut:    "|",
			Category:    CategoryNavigation,
			Aliases:     []string{"quick switch", "task switcher", "recent visits"},
			Handler: func() tea.Msg {
				if cmd := m.toggleQuickSwitch(); cmd != nil {
					return PaletteCommandResultMsg{Cmd: cmd}
				}
				return nil
			},
		},
		{
			Name:        "peek task detail",
			Description: "Peek at the selected task's details",
			Shortcut:    "shift+enter",
			Category:    CategoryNavigation,
			Views:       taskPeekViews(),
			Aliases:     []string{"peek task", "task peek", "preview task"},
			Handler: func() tea.Msg {
				task, project := m.peekCursorTaskAndProject()
				if task == nil {
					return nil
				}
				// Return the shell-owned overlay message (as the palette task-row
				// Peek does) instead of mutating peek directly and returning nil.
				// CommandPaletteExecuteMsg treats a nil result as a non-modal
				// action and pops the palette; PalettePeekRequestMsg is in its
				// keep-palette set, so the palette's query/cursor/selection survive
				// while the same in-app Peek overlay opens as with Shift+Enter.
				return PalettePeekRequestMsg{Task: task, Project: project}
			},
		},
	}
}
