package tui

import (
	tea "charm.land/bubbletea/v2"
)

// syntheticKey constructs a KeyPressMsg for a single rune character.
func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}
