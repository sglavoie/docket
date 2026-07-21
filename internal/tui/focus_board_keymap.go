package tui

import "github.com/sglavoie/tt/internal/tui/keymap"

func (m *FocusBoardModel) buildKeyMap() *keymap.KeyMap {
	return BuildPlannerKeyMap(m, m.Capabilities(), m.handlePlannerAction)
}
