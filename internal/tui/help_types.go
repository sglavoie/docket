package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/pulse"
)

// HelpItem represents a single help entry. Aliases carry the vocabulary a user
// is likely to type for an operation whose label does not contain it — "defer"
// for Set Due or Defer — so `?` search can reach a row its label hides. They are
// matched, never rendered as a column of their own.
type HelpItem struct {
	Key         string // e.g., "j/k", "ctrl+O"
	Description string // e.g., "Move cursor"
	Category    string // e.g., "Navigation", "Actions"
	Aliases     []string
}

// filteredHelpItem pairs a HelpItem with fuzzy match positions for highlighting.
// matchedAliases names the aliases that earned the match, so the row can say why
// it survived a query its description does not contain.
type filteredHelpItem struct {
	HelpItem
	keyPositions   []int
	descPositions  []int
	matchedAliases []string
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
	pulse.WorkspacePulseState
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
	viewName          string
	ready             bool
	activeTab         int      // Current tab (category) index
	categories        []string // Ordered list of categories derived from items
	tabSearchMode     bool     // True when tab search input is active
	tabSearchInput    textinput.Model
}
