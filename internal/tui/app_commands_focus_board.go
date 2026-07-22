package tui

import tea "charm.land/bubbletea/v2"

// focusBoardCommands exposes the consolidated Planner command catalog. Most
// actions route back through the Planner dispatcher the direct keys use. The
// three shell-owned actions AppModel intercepts before delegation (Peek, Undo,
// Redo) route through shellOwnedPaletteAction instead, so the palette matches the
// direct Shift+Enter / u / Ctrl+R keys rather than the dispatcher's stand-ins
// (handlePlannerAction folds Peek into Enter and no-ops Undo/Redo).
func (m *AppModel) focusBoardCommands() []Command {
	return PlannerCommands(func(action PlannerAction) tea.Msg {
		planner, ok := m.views.Get(ViewFocusBoard).(*FocusBoardModel)
		if !ok || planner == nil {
			return nil
		}
		task, project := planner.getTaskRef()
		if msg, shellOwned := m.shellOwnedPaletteAction(action, task, project); shellOwned {
			return msg
		}
		cmd := planner.handlePlannerAction(action)
		if cmd == nil {
			return nil
		}
		return PaletteCommandResultMsg{Cmd: cmd}
	})
}

func (m *AppModel) focusBoardSuggestedCommandOrder() []string {
	return []string{
		"Planner Open task detail",
		"Planner Complete task",
		"Planner Add task",
		"Planner Search tasks",
		"Planner Open View menu",
		"Planner Next Planner tab",
	}
}
