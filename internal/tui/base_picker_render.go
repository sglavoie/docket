package tui

import (
	"fmt"
	"strings"
)

// RenderList renders the scrollable item list with cursor and selection indicators.
func (p *BasePicker[T]) RenderList(maxVisible int) string {
	if maxVisible < 5 {
		maxVisible = 5
	}

	var b strings.Builder

	if len(p.Filtered) == 0 {
		if len(p.Items) == 0 {
			b.WriteString(MutedStyle.Render(p.Config.EmptyMessage))
		} else {
			b.WriteString(MutedStyle.Render(p.Config.NoMatchMessage))
		}
		b.WriteString("\n")
		return b.String()
	}

	// Calculate scroll window
	start := 0
	if p.Cursor >= maxVisible {
		start = p.Cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(p.Filtered))

	// Determine width for selection row
	rowWidth := p.Width()
	if rowWidth <= 0 {
		rowWidth = 80
	}
	rowWidth -= 4 // Account for padding

	for i := start; i < end; i++ {
		item := p.Filtered[i]
		selected := i == p.Cursor

		// Check if item is pinned (optional interface)
		isPinned := false
		if pinnable, ok := any(item).(PinnablePickerItem); ok {
			isPinned = pinnable.IsPinned()
		}

		// Checkbox for multi-select
		checkbox := ""
		if p.Config.MultiSelect {
			if p.Selected[item.ItemID()] {
				checkbox = SelectionCheckStyle.Render("[x]") + " "
			} else {
				checkbox = MutedStyle.Render("[ ]") + " "
			}
		}

		// Display text with optional pinned styling
		displayText := item.DisplayText()
		if isPinned {
			if selected {
				displayText = SuccessBoldStyle.Render(displayText)
			} else {
				displayText = PinnedProjectStyle.Render(displayText)
			}
		}

		// Count (optional)
		countStr := ""
		if p.Config.ShowCounts {
			if cd, ok := any(item).(CountDisplayer); ok {
				countStr = " " + cd.CountDisplayText()
			} else {
				countStr = TagManagerCountStyle.Render(fmt.Sprintf(" (%d)", item.CountValue()))
			}
		}

		// Build content without cursor
		content := checkbox + displayText + countStr

		// Apply selection styling
		line := RenderSelectableRow(selected, false, false, false, false, false, content, rowWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(p.Filtered) > maxVisible {
		remaining := len(p.Filtered) - end
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderSearchBar renders the search input when active.
func (p *BasePicker[T]) RenderSearchBar() string {
	if !p.IsSearchMode() {
		return ""
	}
	return "Filter: " + p.SearchInputView() + "\n\n"
}

// RenderFooter renders context-appropriate keyboard hints.
func (p *BasePicker[T]) RenderFooter() string {
	if p.IsSearchMode() {
		return MutedStyle.Render("enter done · esc clear")
	}

	if len(p.Filtered) == 0 {
		return MutedStyle.Render("esc cancel")
	}

	if p.Config.MultiSelect {
		return MutedStyle.Render("space toggle · / search · c clear · enter apply · esc cancel")
	}

	return MutedStyle.Render("enter select · / search · esc cancel")
}

// RenderInBox renders the complete picker UI as a fullscreen view with title and footer.
func (p *BasePicker[T]) RenderInBox(titleContent, headerContent string, maxVisible int) string {
	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle(titleContent, headerContent))
	b.WriteString("\n\n")

	// Search bar
	b.WriteString(p.RenderSearchBar())

	// List
	if maxVisible == 0 {
		maxVisible = 15
		if p.Height() > 0 {
			maxVisible = p.Height() - 8
		}
	}
	b.WriteString(p.RenderList(maxVisible))

	// Footer
	left := HelpStyle.Render(p.RenderFooter())
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(p.Filtered)))
	b.WriteString(renderStatusBarFooter(left, right, p.Width()))

	return b.String()
}

// pluralize returns singular if count is 1, plural otherwise.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
