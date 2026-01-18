package tui

import "strings"

// PrefixHint represents a single key hint in a prefix popup.
type PrefixHint struct {
	Key         string
	Description string
}

// PrefixHints maps prefix characters to their available key hints.
type PrefixHints map[string][]PrefixHint

// RenderPrefixPopup renders a which-key style popup showing available keys for a prefix.
// Returns empty string if prefix is empty or not found in hints.
func RenderPrefixPopup(prefix string, hints PrefixHints) string {
	if prefix == "" {
		return ""
	}

	keyHints, ok := hints[prefix]
	if !ok || len(keyHints) == 0 {
		return ""
	}

	var parts []string
	for _, hint := range keyHints {
		parts = append(parts, PrefixDescStyle.Render(hint.Description)+PrefixKeyStyle.Render("("+hint.Key+")"))
	}

	hintsStr := strings.Join(parts, " ")
	return PrefixPopupStyle.Render(prefix + " → " + hintsStr + "  " + MutedStyle.Render("Esc to cancel"))
}

// Common prefix hints used across multiple views.
var (
	// StatusPrefixHints contains hints for the "," (status) prefix.
	StatusPrefixHints = []PrefixHint{
		{Key: "s", Description: "start"},
		{Key: "d", Description: "done"},
		{Key: "b", Description: "block"},
		{Key: "x", Description: "cancel"},
		{Key: "o", Description: "reopen"},
	}

	// FoldPrefixHints contains hints for the "z" (fold) prefix.
	FoldPrefixHints = []PrefixHint{
		{Key: "a", Description: "toggle"},
		{Key: "o", Description: "open"},
		{Key: "c", Description: "close"},
		{Key: "R", Description: "expand-all"},
		{Key: "M", Description: "collapse-all"},
		{Key: "v", Description: "focus"},
	}

	// NavigationPrefixHints contains hints for the "g" (go/navigation) prefix - basic version.
	NavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first"},
	}

	// ProjectTreeNavigationPrefixHints contains extended "g" prefix hints for project tree.
	ProjectTreeNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first"},
		{Key: "a", Description: "active"},
		{Key: "b", Description: "backlog"},
		{Key: "i", Description: "inbox"},
		{Key: "c", Description: "completed"},
		{Key: "A", Description: "→Active"},
		{Key: "B", Description: "→Backlog"},
		{Key: "I", Description: "→Inbox"},
		{Key: "d", Description: "due-filter"},
		{Key: "v", Description: "view-cycle"},
		{Key: "X", Description: "deselect"},
	}
)
