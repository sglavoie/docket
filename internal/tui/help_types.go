package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// HelpItem represents a single help entry.
type HelpItem struct {
	Key         string // e.g., "j/k", "ctrl+O"
	Description string // e.g., "Move cursor"
	Category    string // e.g., "Navigation", "Actions"
}

// filteredHelpItem pairs a HelpItem with fuzzy match positions for highlighting.
type filteredHelpItem struct {
	HelpItem
	keyPositions  []int
	descPositions []int
}

// HelpProvider is implemented by views that project help directly from their
// active keymap or other view-local controls.
type HelpProvider interface {
	GetHelpItems() []HelpItem
}

func helpItemsFromKeyMap(km *keymap.KeyMap) []HelpItem {
	raw := km.HelpItems()
	items := make([]HelpItem, len(raw))
	for i, item := range raw {
		items[i] = HelpItem{
			Key:         item.Key,
			Description: item.Description,
			Category:    item.Category,
		}
	}
	return items
}

// HelpViewModel is the model for the help view.
type HelpViewModel struct {
	workspacePulseState
	items         []HelpItem
	filteredItems []filteredHelpItem
	viewport      viewport.Model
	searchInput   textinput.Model
	searchMode    bool
	searchQuery   string
	// searchCaseRelaxed records that the filtered items came from a
	// case-insensitive retry after the query's smart-case reading matched nothing.
	searchCaseRelaxed bool
	width             int
	sourceView        PresentationID
	viewName          string
	ready             bool
	activeTab         int      // Current tab (category) index
	categories        []string // Ordered list of categories derived from items
	tabSearchMode     bool     // True when tab search input is active
	tabSearchInput    textinput.Model
}
