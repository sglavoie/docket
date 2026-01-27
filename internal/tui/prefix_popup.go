package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PrefixHint represents a single key hint in a prefix popup.
type PrefixHint struct {
	Key         string
	Description string
}

// PrefixHints maps prefix characters to their available key hints.
type PrefixHints map[string][]PrefixHint

// RenderPrefixPopup renders a which-key style popup showing available keys for a prefix.
// Wraps hints to multiple lines if they exceed the given width.
// Returns empty string if prefix is empty or not found in hints.
func RenderPrefixPopup(prefix string, hints PrefixHints, width int) string {
	if prefix == "" {
		return ""
	}

	keyHints, ok := hints[prefix]
	if !ok || len(keyHints) == 0 {
		return ""
	}

	// Build individual hint parts
	var parts []string
	for _, hint := range keyHints {
		parts = append(parts, PrefixDescStyle.Render(hint.Description)+PrefixKeyStyle.Render("("+hint.Key+")"))
	}

	// Calculate prefix and suffix
	linePrefix := prefix + " → "
	lineSuffix := "  " + MutedStyle.Render("Esc to cancel")
	continuationIndent := "    " // Indent for wrapped lines

	// Account for PrefixPopupStyle padding (1 char each side)
	availableWidth := width - 2
	if availableWidth < 20 {
		availableWidth = 20
	}

	// Build lines with wrapping
	var lines []string
	currentLine := linePrefix
	currentLineWidth := lipgloss.Width(linePrefix)

	for i, part := range parts {
		partWidth := lipgloss.Width(part)
		separator := ""
		separatorWidth := 0
		if i > 0 || currentLine != linePrefix {
			separator = " "
			separatorWidth = 1
		}

		// Check if adding this part would exceed width
		if currentLineWidth+separatorWidth+partWidth > availableWidth && currentLine != linePrefix {
			// Wrap to new line
			lines = append(lines, currentLine)
			currentLine = continuationIndent + part
			currentLineWidth = lipgloss.Width(continuationIndent) + partWidth
		} else {
			// Add to current line
			if separator != "" && currentLine != linePrefix {
				currentLine += separator
				currentLineWidth += separatorWidth
			}
			currentLine += part
			currentLineWidth += partWidth
		}
	}

	// Add suffix to last line if it fits, otherwise add new line
	suffixWidth := lipgloss.Width(lineSuffix)
	if currentLineWidth+suffixWidth <= availableWidth {
		currentLine += lineSuffix
	} else {
		lines = append(lines, currentLine)
		currentLine = continuationIndent + strings.TrimSpace(lineSuffix)
	}
	lines = append(lines, currentLine)

	// Render each line with the popup style
	var renderedLines []string
	for _, line := range lines {
		renderedLines = append(renderedLines, PrefixPopupStyle.Render(line))
	}

	return strings.Join(renderedLines, "\n")
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

	// PriorityViewNavigationPrefixHints contains "g" prefix hints for priority view.
	PriorityViewNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first"},
		{Key: "t", Description: "tree"},
	}

	// ProjectTreeNavigationPrefixHints contains extended "g" prefix hints for project tree.
	ProjectTreeNavigationPrefixHints = []PrefixHint{
		{Key: "g", Description: "first"},
		{Key: "a", Description: "active"},
		{Key: "n", Description: "next"},
		{Key: "i", Description: "inbox"},
		{Key: "w", Description: "waiting"},
		{Key: "s", Description: "someday"},
		{Key: "c", Description: "completed"},
		{Key: "A", Description: "→Active"},
		{Key: "N", Description: "→Next"},
		{Key: "I", Description: "→Inbox"},
		{Key: "W", Description: "→Waiting"},
		{Key: "S", Description: "→Someday"},
		{Key: "d", Description: "due-filter"},
		{Key: "v", Description: "view-cycle"},
		{Key: "X", Description: "deselect"},
	}

	// PriorityPrefixHints contains hints for the "!" (priority) prefix.
	PriorityPrefixHints = []PrefixHint{
		{Key: "1", Description: "critical"},
		{Key: "2", Description: "high"},
		{Key: "3", Description: "normal"},
		{Key: "4", Description: "low"},
		{Key: "5", Description: "someday"},
	}

	// DueDatePrefixHints contains hints for the "d" (due date) prefix.
	DueDatePrefixHints = []PrefixHint{
		{Key: "t", Description: "today"},
		{Key: "m", Description: "tomorrow"},
		{Key: "w", Description: "+week"},
		{Key: "n", Description: "+month"},
		{Key: "0", Description: "clear"},
		{Key: "d", Description: "picker"},
	}

	// SectionPrefixHints contains hints for the "s" (section focus) prefix in Review view.
	SectionPrefixHints = []PrefixHint{
		{Key: "d", Description: "overdue"},
		{Key: "t", Description: "today"},
		{Key: "s", Description: "due-soon"},
		{Key: "l", Description: "due-later"},
		{Key: "f", Description: "future"},
		{Key: "o", Description: "old"},
	}

	// CreateLinkedPrefixHints contains hints for the "c" (create linked) prefix.
	CreateLinkedPrefixHints = []PrefixHint{
		{Key: "b", Description: "blocker"},
		{Key: "d", Description: "dependent"},
		{Key: "s", Description: "subtask"},
		{Key: "f", Description: "follow-up"},
		{Key: "n", Description: "navigate"},
	}
)
