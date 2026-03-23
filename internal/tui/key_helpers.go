package tui

import (
	tea "charm.land/bubbletea/v2"
)

// syntheticKey constructs a KeyPressMsg for a single rune character.
func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// syntheticSpecialKey constructs a KeyPressMsg for a special key.
func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// syntheticKeyStr constructs a KeyPressMsg from a string.
func syntheticKeyStr(s string) tea.KeyPressMsg {
	runes := []rune(s)
	if len(runes) == 1 {
		return tea.KeyPressMsg{Code: runes[0], Text: s}
	}
	return tea.KeyPressMsg{Text: s}
}

// syntheticCtrlKey constructs a KeyPressMsg for a ctrl+key combination.
// r should be the base key rune (e.g. 'g' for ctrl+g).
func syntheticCtrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}
