package tui

import tea "charm.land/bubbletea/v2"

func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func syntheticKeyStr(s string) tea.KeyPressMsg {
	runes := []rune(s)
	if len(runes) == 1 {
		return tea.KeyPressMsg{Code: runes[0], Text: s}
	}
	return tea.KeyPressMsg{Text: s}
}

func syntheticCtrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}
