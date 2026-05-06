package tui

import (
	tea "charm.land/bubbletea/v2"
)

// SectionMoveCallbacks provides the view-specific operations needed by
// handleSectionMovePrefix. Each primary view populates this with its own
// move methods so the dispatch logic can be shared.
type SectionMoveCallbacks struct {
	// MoveToSection moves the cursor task to the given section.
	MoveToSection func(section string) (tea.Model, tea.Cmd)
	// BulkMoveToSection moves all selected tasks to the given section.
	BulkMoveToSection func(section string) (tea.Model, tea.Cmd)
	// MoveToTop moves the cursor task to the top of its group.
	MoveToTop func() (tea.Model, tea.Cmd)
	// MoveToBottom moves the cursor task to the bottom of its group.
	MoveToBottom func() (tea.Model, tea.Cmd)
	// IsMultiSelect returns true when multi-select mode is active with selections.
	IsMultiSelect func() bool
	// CanMoveSection returns true when section moves are allowed (e.g. Outliner
	// only allows them in section group-by mode). Nil means always allowed.
	CanMoveSection func() bool
}

// handleSectionMovePrefix dispatches the m-prefix key to the appropriate
// section move or position move callback. Returns (model, cmd, handled).
func handleSectionMovePrefix(key string, cb SectionMoveCallbacks) (tea.Model, tea.Cmd, bool) {
	if section, ok := sectionForMoveKey(key); ok {
		if cb.CanMoveSection != nil && !cb.CanMoveSection() {
			return nil, nil, true
		}
		if cb.IsMultiSelect != nil && cb.IsMultiSelect() {
			model, cmd := cb.BulkMoveToSection(section)
			return model, cmd, true
		}
		model, cmd := cb.MoveToSection(section)
		return model, cmd, true
	}
	switch key {
	case "K":
		model, cmd := cb.MoveToTop()
		return model, cmd, true
	case "J":
		model, cmd := cb.MoveToBottom()
		return model, cmd, true
	}
	return nil, nil, false
}

// stageKeyForIndex returns the shortcut key for a stage at the given index.
// Indices 0-8 map to "1"-"9", indices 9-34 map to "a"-"z", beyond returns "".
func stageKeyForIndex(i int) string {
	if i >= 0 && i < 9 {
		return string(rune('1' + i))
	}
	if i >= 9 && i < 35 {
		return string(rune('a' + i - 9))
	}
	return ""
}

// stageIndexForKey returns the stage index for a shortcut key, or -1 if unrecognized.
// "1"-"9" map to 0-8, "a"-"z" map to 9-34.
func stageIndexForKey(key string) int {
	if len(key) != 1 {
		return -1
	}
	ch := key[0]
	if ch >= '1' && ch <= '9' {
		return int(ch - '1')
	}
	if ch >= 'a' && ch <= 'z' {
		return int(ch-'a') + 9
	}
	return -1
}

// StageForPrefixKey maps a key press to a stage name from the configured stages.
// Keys 1-9 map to stages[0..8], a-z map to stages[9..34], key 0 clears (empty string).
// Returns nil if the key is not recognized.
func StageForPrefixKey(key string, stages []string) *string {
	if key == "0" {
		s := ""
		return &s
	}
	idx := stageIndexForKey(key)
	if idx >= 0 && idx < len(stages) {
		return &stages[idx]
	}
	return nil
}
