package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PickerItem defines the interface that any picker item must implement.
type PickerItem interface {
	// FilterText returns the text used for case-insensitive filtering.
	FilterText() string
	// DisplayText returns the primary text to display for this item.
	DisplayText() string
	// CountValue returns an optional count to display (0 if not applicable).
	CountValue() int
	// ItemID returns a unique identifier for selection tracking.
	ItemID() string
}

// PickerConfig holds customizable settings for a picker.
type PickerConfig struct {
	Title          string // Title displayed at the top
	Subtitle       string // Optional subtitle (e.g., current selection info)
	EmptyMessage   string // Message when no items exist
	NoMatchMessage string // Message when filter matches nothing
	MultiSelect    bool   // Whether multiple items can be selected
	ShowCounts     bool   // Whether to show count values
	BoxWidth       int    // Width of the picker box (0 for default)
}

// BasePicker provides shared functionality for all picker components.
// It handles search, filtering, cursor navigation, and rendering.
type BasePicker[T PickerItem] struct {
	BaseSearchableView
	Config   PickerConfig
	Items    []T
	Filtered []T
	Selected map[string]bool // Only used when MultiSelect=true
	Cursor   int
}

// NewBasePicker creates a new BasePicker with the given items and config.
func NewBasePicker[T PickerItem](items []T, config PickerConfig) *BasePicker[T] {
	p := &BasePicker[T]{
		Config:   config,
		Items:    items,
		Filtered: items,
		Selected: make(map[string]bool),
		Cursor:   0,
	}
	p.InitSearchInput("Type to filter...", 50, 30)
	return p
}

// SetItems updates the item list and resets the filter.
func (p *BasePicker[T]) SetItems(items []T) {
	p.Items = items
	p.Filter(p.SearchQuery())
}

// Filter updates the filtered list based on the search query.
func (p *BasePicker[T]) Filter(query string) {
	if query == "" {
		p.Filtered = p.Items
	} else {
		queryLower := strings.ToLower(query)
		var filtered []T
		for _, item := range p.Items {
			if strings.Contains(strings.ToLower(item.FilterText()), queryLower) {
				filtered = append(filtered, item)
			}
		}
		p.Filtered = filtered
	}
	if p.Cursor >= len(p.Filtered) {
		p.Cursor = 0
	}
}

// HandleKeyMsg processes keyboard input for navigation and selection.
// Returns (handled, cmd) where handled indicates if the key was consumed.
func (p *BasePicker[T]) HandleKeyMsg(msg tea.KeyMsg) (bool, tea.Cmd) {
	// Handle search mode first
	if p.IsSearchMode() {
		handled, cmd, valueChanged := p.HandleSearchInput(msg)
		if valueChanged {
			p.Filter(p.SearchQuery())
		}
		if handled {
			return true, cmd
		}
		return false, cmd
	}

	// Navigation and selection
	switch msg.String() {
	case "j", "down":
		p.Cursor = WrapCursor(p.Cursor, len(p.Filtered), 1)
		return true, nil

	case "k", "up":
		p.Cursor = WrapCursor(p.Cursor, len(p.Filtered), -1)
		return true, nil

	case "/":
		return true, p.ActivateSearch()

	case " ":
		if p.Config.MultiSelect && len(p.Filtered) > 0 && p.Cursor < len(p.Filtered) {
			item := p.Filtered[p.Cursor]
			id := item.ItemID()
			if p.Selected[id] {
				delete(p.Selected, id)
			} else {
				p.Selected[id] = true
			}
			return true, nil
		}
		return false, nil

	case "c":
		if p.Config.MultiSelect {
			p.Selected = make(map[string]bool)
			return true, nil
		}
		return false, nil
	}

	return false, nil
}

// SelectedItem returns the currently highlighted item (for single-select pickers).
// Returns the zero value if no items are available.
func (p *BasePicker[T]) SelectedItem() T {
	var zero T
	if len(p.Filtered) == 0 || p.Cursor >= len(p.Filtered) {
		return zero
	}
	return p.Filtered[p.Cursor]
}

// SelectedItems returns all selected items (for multi-select pickers).
func (p *BasePicker[T]) SelectedItems() []T {
	var result []T
	for _, item := range p.Items {
		if p.Selected[item.ItemID()] {
			result = append(result, item)
		}
	}
	return result
}

// SelectedIDs returns the IDs of all selected items.
func (p *BasePicker[T]) SelectedIDs() []string {
	var ids []string
	for id := range p.Selected {
		ids = append(ids, id)
	}
	return ids
}

// IsSelected returns whether an item is currently selected.
func (p *BasePicker[T]) IsSelected(item T) bool {
	return p.Selected[item.ItemID()]
}

// PreSelectByID marks an item as selected by its ID.
func (p *BasePicker[T]) PreSelectByID(id string) {
	p.Selected[id] = true
}

// SetCursorToID moves the cursor to the item with the given ID.
func (p *BasePicker[T]) SetCursorToID(id string) {
	for i, item := range p.Filtered {
		if item.ItemID() == id {
			p.Cursor = i
			return
		}
	}
}

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
	end := start + maxVisible
	if end > len(p.Filtered) {
		end = len(p.Filtered)
	}

	// Determine width for selection row (use box width or default)
	rowWidth := p.Config.BoxWidth
	if rowWidth == 0 {
		rowWidth = 50
	}
	if p.Width() > 0 && p.Width() < rowWidth+10 {
		rowWidth = p.Width() - 10
	}
	rowWidth -= 6 // Account for box padding/borders

	for i := start; i < end; i++ {
		item := p.Filtered[i]
		selected := i == p.Cursor

		// Checkbox for multi-select
		checkbox := ""
		if p.Config.MultiSelect {
			if p.Selected[item.ItemID()] {
				checkbox = SelectionCheckStyle.Render("[x]") + " "
			} else {
				checkbox = MutedStyle.Render("[ ]") + " "
			}
		}

		// Display text
		displayText := item.DisplayText()

		// Count (optional)
		countStr := ""
		if p.Config.ShowCounts {
			countStr = TagManagerCountStyle.Render(fmt.Sprintf(" (%d)", item.CountValue()))
		}

		// Build content without cursor
		content := checkbox + displayText + countStr

		// Apply selection styling
		line := RenderSelectableRow(selected, content, rowWidth)
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
		return MutedStyle.Render("enter done | esc clear")
	}

	if len(p.Filtered) == 0 {
		return MutedStyle.Render("esc cancel")
	}

	if p.Config.MultiSelect {
		return MutedStyle.Render("space toggle | / search | c clear | enter apply | esc cancel")
	}

	return MutedStyle.Render("enter select | / search | esc cancel")
}

// RenderInBox renders the complete picker UI in a centered box with titled border.
func (p *BasePicker[T]) RenderInBox(titleContent, headerContent string, maxVisible int) string {
	boxWidth := p.Config.BoxWidth
	if boxWidth == 0 {
		boxWidth = 50
	}
	if p.Width() > 0 && p.Width() < boxWidth+10 {
		boxWidth = p.Width() - 10
	}

	var content strings.Builder

	// Search bar
	content.WriteString(p.RenderSearchBar())

	// List
	if maxVisible == 0 {
		maxVisible = 15
		if p.Height() > 0 {
			maxVisible = p.Height() - 14
		}
	}
	content.WriteString(p.RenderList(maxVisible))

	// Footer
	content.WriteString("\n")
	content.WriteString(p.RenderFooter())

	return renderOverlayCard(content.String(), p.Width(), p.Height(), OverlayCardConfig{
		Title:    titleContent,
		Subtitle: headerContent,
		Width:    boxWidth,
		Centered: true,
	})
}

// pluralize returns singular if count is 1, plural otherwise.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
