package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
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

// PinnablePickerItem is an optional interface for items that can be pinned.
// Items implementing this interface will be rendered with pinned styling.
type PinnablePickerItem interface {
	IsPinned() bool
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
func (p *BasePicker[T]) HandleKeyMsg(msg tea.KeyPressMsg) (bool, tea.Cmd) {
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

	case "space":
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
