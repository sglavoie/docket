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
//   - pages: "pgup"/"pageup", "pgdown"/"pagedown"
//   - edges: "home", "end"
//   - menu function key: "f10"
//   - modifier prefixes joined with "+": "ctrl+x", "alt+x", "shift+tab"
//   - modifier and lock keys pressed alone: "leftshift", "rightctrl", "capslock"
//
// Modifiers may be combined (e.g. "ctrl+shift+a"). The base key is the final
// "+"-separated segment. An error is returned for empty input, unknown
// modifiers, or unsupported base key names.
func parseUITestKey(name string) (tea.KeyPressMsg, error) {
	if name == "" {
		return tea.KeyPressMsg{}, fmt.Errorf("empty key name")
	}

	// A single rune is the key itself, checked before the modifier split so the
	// one key whose name is the separator — `+`, which steps a deadline — can be
	// spelled at all. Splitting it first yields two empty halves and an error.
	if runes := []rune(name); len(runes) == 1 && runes[0] == '+' {
		return tea.KeyPressMsg{Code: runes[0], Text: name}, nil
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

	if code, selfMod, ok := modifierOnlyKeyByName(strings.ToLower(base)); ok {
		return tea.KeyPressMsg{Code: code, Mod: mod | selfMod}, nil
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
	case "pgup", "pageup":
		key.Code = tea.KeyPgUp
	case "pgdown", "pagedown":
		key.Code = tea.KeyPgDown
	case "home":
		key.Code = tea.KeyHome
	case "end":
		key.Code = tea.KeyEnd
	case "f10":
		key.Code = tea.KeyF10
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

// modifierOnlyKeyByName resolves a modifier or lock key pressed by itself, which
// a terminal reporting every key as an escape code sends between a chord prefix
// and the shifted second key that needs it. The names match what Key.String()
// renders, and the returned modifier matches what the decoder sets on the press,
// so a replayed sequence is the one a keyboard actually produces. These keys
// carry no text.
func modifierOnlyKeyByName(name string) (rune, tea.KeyMod, bool) {
	switch name {
	case "leftshift":
		return tea.KeyLeftShift, tea.ModShift, true
	case "rightshift":
		return tea.KeyRightShift, tea.ModShift, true
	case "leftctrl":
		return tea.KeyLeftCtrl, tea.ModCtrl, true
	case "rightctrl":
		return tea.KeyRightCtrl, tea.ModCtrl, true
	case "leftalt":
		return tea.KeyLeftAlt, tea.ModAlt, true
	case "rightalt":
		return tea.KeyRightAlt, tea.ModAlt, true
	case "leftsuper":
		return tea.KeyLeftSuper, tea.ModSuper, true
	case "rightsuper":
		return tea.KeyRightSuper, tea.ModSuper, true
	case "leftmeta":
		return tea.KeyLeftMeta, tea.ModMeta, true
	case "rightmeta":
		return tea.KeyRightMeta, tea.ModMeta, true
	case "lefthyper":
		return tea.KeyLeftHyper, tea.ModHyper, true
	case "righthyper":
		return tea.KeyRightHyper, tea.ModHyper, true
	case "capslock":
		return tea.KeyCapsLock, tea.ModCapsLock, true
	case "numlock":
		return tea.KeyNumLock, tea.ModNumLock, true
	case "scrolllock":
		return tea.KeyScrollLock, 0, true
	}
	return 0, 0, false
}
