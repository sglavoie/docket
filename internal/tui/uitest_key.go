package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// parseUITestKey converts a human-readable key name into a tea.KeyPressMsg the
// AppModel can process. It supports:
//
//   - printable single runes: "a", "A", "1", "!"
//   - named special keys: "enter", "esc"/"escape", "tab", "space", "backspace"
//   - arrows: "up", "down", "left", "right"
//   - modifier prefixes joined with "+": "ctrl+x", "alt+x", "shift+tab"
//
// Modifiers may be combined (e.g. "ctrl+shift+a"). The base key is the final
// "+"-separated segment. An error is returned for empty input, unknown
// modifiers, or unsupported base key names.
func parseUITestKey(name string) (tea.KeyPressMsg, error) {
	if name == "" {
		return tea.KeyPressMsg{}, fmt.Errorf("empty key name")
	}

	parts := strings.Split(name, "+")
	base := parts[len(parts)-1]
	mods := parts[:len(parts)-1]

	if base == "" {
		return tea.KeyPressMsg{}, fmt.Errorf("missing base key in %q", name)
	}

	var mod tea.KeyMod
	for _, m := range mods {
		switch strings.ToLower(m) {
		case "ctrl":
			mod |= tea.ModCtrl
		case "alt":
			mod |= tea.ModAlt
		case "shift":
			mod |= tea.ModShift
		default:
			return tea.KeyPressMsg{}, fmt.Errorf("unknown modifier %q in %q", m, name)
		}
	}

	key := tea.KeyPressMsg{Mod: mod}
	switch strings.ToLower(base) {
	case "enter", "return":
		key.Code = tea.KeyEnter
	case "esc", "escape":
		key.Code = tea.KeyEscape
	case "tab":
		key.Code = tea.KeyTab
	case "space":
		key.Code = tea.KeySpace
		if mod == 0 {
			key.Text = " "
		}
	case "backspace":
		key.Code = tea.KeyBackspace
	case "up":
		key.Code = tea.KeyUp
	case "down":
		key.Code = tea.KeyDown
	case "left":
		key.Code = tea.KeyLeft
	case "right":
		key.Code = tea.KeyRight
	default:
		runes := []rune(base)
		if len(runes) != 1 {
			return tea.KeyPressMsg{}, fmt.Errorf("unsupported key name %q", name)
		}
		key.Code = runes[0]
		// Text is only meaningful for printable characters without non-shift
		// modifiers (matching how a real terminal reports the key). Modified
		// keys like ctrl+x carry no printable text.
		if mod == 0 {
			key.Text = base
		}
	}

	return key, nil
}
