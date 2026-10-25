package grammar

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// AutocompleteState manages the shared state for trigger-based autocomplete
// dropdowns used across quick capture and task editing,
// and global search inputs.
type AutocompleteState struct {
	Mode        int
	Cursor      int
	ViewportTop int
	TriggerPos  int
	Items       []ACSuggestion
	// VisibleCount overrides the number of suggestion rows shown at once. Zero
	// means use the shared acVisibleCount default (seven), which every consumer
	// keeps unless it explicitly opts into a smaller window (e.g. Quick Capture
	// rendering the dropdown into a bounded lower pane).
	VisibleCount int
}

// visibleCount returns the effective scroll-window size: the caller-provided
// VisibleCount when positive, otherwise the shared acVisibleCount default.
func (ac *AutocompleteState) visibleCount() int {
	if ac.VisibleCount > 0 {
		return ac.VisibleCount
	}
	return acVisibleCount
}

// ACSuggestion represents an autocomplete suggestion with display text and
// a raw value to insert.
type ACSuggestion struct {
	Display string
	Value   string
}

// ACTrigger maps trigger text to an autocomplete mode constant.
type ACTrigger struct {
	Text string
	Mode int
}

// IsActive returns whether the autocomplete dropdown is currently showing.
func (ac *AutocompleteState) IsActive() bool {
	return ac.Mode != ACModeNone && len(ac.Items) > 0
}

// Clear resets all autocomplete state.
func (ac *AutocompleteState) Clear() {
	ac.Mode = ACModeNone
	ac.Items = nil
	ac.Cursor = 0
	ac.ViewportTop = 0
	ac.TriggerPos = 0
	ac.VisibleCount = 0
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
			if ac.Cursor >= ac.ViewportTop+ac.visibleCount() {
				ac.ViewportTop = ac.Cursor - ac.visibleCount() + 1
			}
		}
		return true
	}
	return false
}

// OffersExactly reports whether partial already spells one of the offered
// values, so accepting a suggestion could only restate what was typed. Enter
// submits in that case instead of spending a keypress on a no-op completion.
func (ac *AutocompleteState) OffersExactly(partial string) bool {
	if !ac.IsActive() || partial == "" {
		return false
	}
	for _, item := range ac.Items {
		if item.Value == partial {
			return true
		}
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
// recent trigger. Triggers only activate at the start of input or after
// whitespace. When triggers share a prefix, the longest matching trigger wins.
// Returns the mode, trigger position, and partial text typed after the trigger.
// Returns ACModeNone if no trigger is found.
func DetectTrigger(input string, triggers []ACTrigger) (mode int, pos int, partial string) {
	if len(input) == 0 {
		return ACModeNone, 0, ""
	}

	for i := len(input) - 1; i >= 0; i-- {
		if input[i] == ' ' || input[i] == '\n' {
			break
		}
		atStartOrAfterSpace := i == 0 || input[i-1] == ' ' || input[i-1] == '\n'
		if !atStartOrAfterSpace {
			continue
		}
		var longest ACTrigger
		for _, t := range triggers {
			if t.Text != "" && len(t.Text) > len(longest.Text) && strings.HasPrefix(input[i:], t.Text) {
				longest = t
			}
		}
		if longest.Text != "" {
			return longest.Mode, i, input[i+len(longest.Text):]
		}
	}
	return ACModeNone, 0, ""
}

// UpdateSuggestions sets the suggestion list and clamps cursor/viewport.
// If mode or trigger position changed since the last call, cursor resets to 0.
// If the new suggestion list is empty, mode is reset to ACModeNone.
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
		ac.Mode = ACModeNone
	}
}

// ClampCursor ensures cursor and viewport are within valid bounds.
func (ac *AutocompleteState) ClampCursor() {
	if ac.Cursor >= len(ac.Items) {
		ac.Cursor = max(0, len(ac.Items)-1)
	}
	if ac.Cursor < ac.ViewportTop {
		ac.ViewportTop = ac.Cursor
	} else if ac.Cursor >= ac.ViewportTop+ac.visibleCount() {
		ac.ViewportTop = ac.Cursor - ac.visibleCount() + 1
	}
}

// BuildAcceptedInputAt replaces through a caller-chosen byte offset.
// It replaces the active token only through cursorByte and preserves any text
// after the cursor, so accepting a completion in the middle of a draft never
// discards trailing tokens.
func (ac *AutocompleteState) BuildAcceptedInputAt(input string, cursorByte int, triggerText string) string {
	if !ac.IsActive() {
		return input
	}
	cursorByte = max(ac.TriggerPos, min(cursorByte, len(input)))
	selected := ac.Selected()
	return input[:ac.TriggerPos] + triggerText + selected.Value + " " + input[cursorByte:]
}

// TriggerCharForMode returns the trigger text for the given autocomplete mode,
// using the provided trigger definitions. Returns an empty string if mode is not found.
func TriggerCharForMode(mode int, triggers []ACTrigger) string {
	for _, t := range triggers {
		if t.Mode == mode {
			return t.Text
		}
	}
	return ""
}

// RenderDropdownWithLimit renders the dropdown constrained to at most rowBudget
// total rows, including the "N more" indicators. It spends rows on indicators
// only when the list actually overflows in that direction, allowing every item
// to render whenever the full list fits. The derived item-window size is
// persisted so keyboard navigation keeps the selection visible.
func (ac *AutocompleteState) RenderDropdownWithLimit(indent string, rowBudget int) string {
	if !ac.IsActive() {
		return ""
	}

	budget := max(rowBudget, 1)
	itemCount := len(ac.Items)
	if itemCount <= budget {
		ac.ViewportTop = 0
		ac.VisibleCount = itemCount
		return strings.TrimRight(ac.renderDropdownWindow(indent, false, false), "\n")
	}

	// A one-row pane cannot show both an item and an overflow hint. Keep the
	// selected item visible; larger panes use their spare row(s) for the hints.
	if budget == 1 {
		ac.ViewportTop = ac.Cursor
		ac.VisibleCount = 1
		return strings.TrimRight(ac.renderDropdownWindow(indent, false, false), "\n")
	}

	edgeWindow := budget - 1
	switch {
	case ac.Cursor < edgeWindow:
		ac.ViewportTop = 0
		ac.VisibleCount = edgeWindow
		return strings.TrimRight(ac.renderDropdownWindow(indent, false, true), "\n")
	case ac.Cursor >= itemCount-edgeWindow:
		ac.VisibleCount = edgeWindow
		ac.ViewportTop = itemCount - edgeWindow
		return strings.TrimRight(ac.renderDropdownWindow(indent, true, false), "\n")
	case budget == 2:
		// There is room for one item and one hint. Prefer the top hint in the
		// middle of the list; navigation still reaches every item.
		ac.ViewportTop = ac.Cursor
		ac.VisibleCount = 1
		return strings.TrimRight(ac.renderDropdownWindow(indent, true, false), "\n")
	default:
		ac.VisibleCount = budget - 2
		ac.ViewportTop = max(1, min(ac.ViewportTop, itemCount-ac.VisibleCount-1))
		if ac.Cursor < ac.ViewportTop {
			ac.ViewportTop = ac.Cursor
		}
		if ac.Cursor >= ac.ViewportTop+ac.VisibleCount {
			ac.ViewportTop = ac.Cursor - ac.VisibleCount + 1
		}
		ac.ViewportTop = max(1, min(ac.ViewportTop, itemCount-ac.VisibleCount-1))
		return strings.TrimRight(ac.renderDropdownWindow(indent, true, true), "\n")
	}
}

// RenderDropdown renders the autocomplete suggestion dropdown as a string.
// The indent parameter controls the leading whitespace before each line.
func (ac *AutocompleteState) RenderDropdown(indent string) string {
	if !ac.IsActive() {
		return ""
	}
	visibleEnd := min(ac.ViewportTop+ac.visibleCount(), len(ac.Items))
	return ac.renderDropdownWindow(indent, ac.ViewportTop > 0, visibleEnd < len(ac.Items))
}

func (ac *AutocompleteState) renderDropdownWindow(indent string, showTop, showBottom bool) string {
	var b strings.Builder
	if showTop {
		b.WriteString(theme.MutedStyle.Render(indent + "↑ " + fmt.Sprintf("%d more", ac.ViewportTop)))
		b.WriteString("\n")
	}

	visibleEnd := min(ac.ViewportTop+ac.visibleCount(), len(ac.Items))

	for i := ac.ViewportTop; i < visibleEnd; i++ {
		s := ac.Items[i]
		if i == ac.Cursor {
			b.WriteString(theme.SelectedStyle.Render(indent + "> " + s.Display))
		} else {
			b.WriteString(theme.MutedStyle.Render(indent + "  " + s.Display))
		}
		b.WriteString("\n")
	}

	remaining := len(ac.Items) - visibleEnd
	if showBottom && remaining > 0 {
		b.WriteString(theme.MutedStyle.Render(indent + "↓ " + fmt.Sprintf("%d more", remaining)))
		b.WriteString("\n")
	}

	return b.String()
}
