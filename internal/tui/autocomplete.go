package tui

import (
	"fmt"
	"strings"
)

// AutocompleteState manages the shared state for trigger-based autocomplete
// dropdowns used across quick capture, kanban quick add, focus board add,
// and global search inputs.
type AutocompleteState struct {
	Mode        int
	Cursor      int
	ViewportTop int
	TriggerPos  int
	Items       []ACSuggestion
}

// ACSuggestion represents an autocomplete suggestion with display text and
// a raw value to insert.
type ACSuggestion struct {
	Display string
	Value   string
}

// ACTrigger maps a trigger character to an autocomplete mode constant.
type ACTrigger struct {
	Char byte
	Mode int
}

// IsActive returns whether the autocomplete dropdown is currently showing.
func (ac *AutocompleteState) IsActive() bool {
	return ac.Mode != acModeNone && len(ac.Items) > 0
}

// Clear resets all autocomplete state.
func (ac *AutocompleteState) Clear() {
	ac.Mode = acModeNone
	ac.Items = nil
	ac.Cursor = 0
	ac.ViewportTop = 0
	ac.TriggerPos = 0
}

// HandleKey handles navigation keys when autocomplete is active.
// Returns true if the key was consumed. The onAccept callback is called
// when tab/enter is pressed (after state is updated). The caller should
// call AcceptValue() inside onAccept to get the selected value and update
// the input field.
func (ac *AutocompleteState) HandleKey(key string, onAccept func()) bool {
	if !ac.IsActive() {
		return false
	}

	switch key {
	case "tab", "enter":
		if onAccept != nil {
			onAccept()
		}
		return true
	case "esc":
		ac.Clear()
		return true
	case "up", "ctrl+p", "ctrl+k":
		if ac.Cursor > 0 {
			ac.Cursor--
			if ac.Cursor < ac.ViewportTop {
				ac.ViewportTop = ac.Cursor
			}
		}
		return true
	case "down", "ctrl+n", "ctrl+j":
		if ac.Cursor < len(ac.Items)-1 {
			ac.Cursor++
			if ac.Cursor >= ac.ViewportTop+acVisibleCount {
				ac.ViewportTop = ac.Cursor - acVisibleCount + 1
			}
		}
		return true
	}
	return false
}

// Selected returns the currently selected suggestion, or a zero value if none.
func (ac *AutocompleteState) Selected() ACSuggestion {
	if ac.Cursor < len(ac.Items) {
		return ac.Items[ac.Cursor]
	}
	return ACSuggestion{}
}

// DetectTrigger scans the input string backward from the end to find the most
// recent trigger character. Triggers only activate at the start of input or
// after whitespace. Returns the mode, trigger position, and partial text typed
// after the trigger. Returns acModeNone if no trigger is found.
func DetectTrigger(input string, triggers []ACTrigger) (mode int, pos int, partial string) {
	if len(input) == 0 {
		return acModeNone, 0, ""
	}

	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			break
		}
		atStartOrAfterSpace := i == 0 || input[i-1] == ' ' || input[i-1] == '\n'
		if !atStartOrAfterSpace {
			continue
		}
		for _, t := range triggers {
			if ch == t.Char {
				return t.Mode, i, input[i+1:]
			}
		}
	}
	return acModeNone, 0, ""
}

// UpdateSuggestions sets the suggestion list and clamps cursor/viewport.
// If mode or trigger position changed since the last call, cursor resets to 0.
// If the new suggestion list is empty, mode is reset to acModeNone.
func (ac *AutocompleteState) UpdateSuggestions(mode, triggerPos int, items []ACSuggestion) {
	if mode != ac.Mode || triggerPos != ac.TriggerPos {
		ac.Cursor = 0
		ac.ViewportTop = 0
	}
	ac.Mode = mode
	ac.TriggerPos = triggerPos
	ac.Items = items

	ac.ClampCursor()

	if len(ac.Items) == 0 {
		ac.Mode = acModeNone
	}
}

// ClampCursor ensures cursor and viewport are within valid bounds.
func (ac *AutocompleteState) ClampCursor() {
	if ac.Cursor >= len(ac.Items) {
		ac.Cursor = max(0, len(ac.Items)-1)
	}
	if ac.Cursor < ac.ViewportTop {
		ac.ViewportTop = ac.Cursor
	} else if ac.Cursor >= ac.ViewportTop+acVisibleCount {
		ac.ViewportTop = ac.Cursor - acVisibleCount + 1
	}
}

// BuildAcceptedInput constructs the new input value after accepting a suggestion.
// It replaces from the trigger position to the end of the input with
// triggerChar + selected.Value + " ".
func (ac *AutocompleteState) BuildAcceptedInput(input string, triggerChar byte) string {
	if !ac.IsActive() {
		return input
	}
	selected := ac.Selected()
	return input[:ac.TriggerPos] + string(triggerChar) + selected.Value + " "
}

// TriggerCharForMode returns the trigger character for the given autocomplete mode,
// using the provided trigger definitions. Returns 0 if mode is not found.
func TriggerCharForMode(mode int, triggers []ACTrigger) byte {
	for _, t := range triggers {
		if t.Mode == mode {
			return t.Char
		}
	}
	return 0
}

// RenderDropdown renders the autocomplete suggestion dropdown as a string.
// The indent parameter controls the leading whitespace before each line.
func (ac *AutocompleteState) RenderDropdown(indent string) string {
	if !ac.IsActive() {
		return ""
	}

	var b strings.Builder
	if ac.ViewportTop > 0 {
		b.WriteString(MutedStyle.Render(indent + "↑ " + fmt.Sprintf("%d more", ac.ViewportTop)))
		b.WriteString("\n")
	}

	visibleEnd := ac.ViewportTop + acVisibleCount
	if visibleEnd > len(ac.Items) {
		visibleEnd = len(ac.Items)
	}

	for i := ac.ViewportTop; i < visibleEnd; i++ {
		s := ac.Items[i]
		if i == ac.Cursor {
			b.WriteString(SelectedStyle.Render(indent + "> " + s.Display))
		} else {
			b.WriteString(MutedStyle.Render(indent + "  " + s.Display))
		}
		b.WriteString("\n")
	}

	remaining := len(ac.Items) - visibleEnd
	if remaining > 0 {
		b.WriteString(MutedStyle.Render(indent + "↓ " + fmt.Sprintf("%d more", remaining)))
		b.WriteString("\n")
	}

	return b.String()
}
