package tui

import tea "charm.land/bubbletea/v2"

// focusBoardCommands exposes the consolidated Planner command catalog.
func (m *AppModel) focusBoardCommands() []Command {
	return PlannerCommands(func(action TaskWorkspaceAction) tea.Msg {
		planner, ok := m.views.Get(ViewFocusBoard).(*FocusBoardModel)
		if !ok || planner == nil {
			return nil
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
