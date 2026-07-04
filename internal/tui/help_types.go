package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
)

// HelpItem represents a single help entry.
type HelpItem struct {
	Key         string     // e.g., "j/k", "ctrl+O"
	Description string     // e.g., "Move cursor"
	Category    string     // e.g., "Navigation", "Actions"
	Views       []ViewType // nil = global; populated by GetAllDeduplicated for explorer scope display
}

// filteredHelpItem pairs a HelpItem with fuzzy match positions for highlighting.
type filteredHelpItem struct {
	HelpItem
	keyPositions  []int
	descPositions []int
}

// HelpProvider is an interface for views that can provide help items.
// Two patterns are acceptable:
//  1. Registry-backed (most views): return GetKeybindingRegistry().GetForView(ViewXxx)
//  2. Custom (views with dynamic descriptions): build and return []HelpItem directly.
//
// Use custom only when descriptions must change at runtime (e.g., toggling archive state).
type HelpProvider interface {
	GetHelpItems() []HelpItem
}

// HelpViewModel is the model for the help view.
type HelpViewModel struct {
	items          []HelpItem
	filteredItems  []filteredHelpItem
	viewport       viewport.Model
	searchInput    textinput.Model
	searchMode     bool
	searchQuery    string
	width          int
	height         int
	sourceView     ViewType
	viewName       string
	ready          bool
	activeTab      int      // Current tab (category) index
	categories     []string // Ordered list of categories derived from items
	isExplorer     bool     // True when used as global keybinding explorer
	byPrefixMode   bool     // True when grouping by prefix category (explorer only)
	tabSearchMode  bool     // True when tab search input is active
	tabSearchInput textinput.Model
	compactTabs    bool // True when explorer category tabs collapse to a one-line summary
}
