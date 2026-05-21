package tui

import "github.com/sglavoie/tt/internal/tui/keymap"

func (m *WeeklyReviewModel) buildKeyMap() *keymap.KeyMap {
	return buildWeeklyReviewKeyMap(m)
}
