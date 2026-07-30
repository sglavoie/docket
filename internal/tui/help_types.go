package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// HelpItem represents a single help entry.
type HelpItem struct {
	Key         string           // e.g., "j/k", "ctrl+O"
	Description string           // e.g., "Move cursor"
	Category    string           // e.g., "Navigation", "Actions"
	Views       []PresentationID // nil = global; populated by GetAllDeduplicated for explorer scope display
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

// getGlobalHelpItems projects the descriptor-backed portion of the accepted
// root grammar for legacy secondary views that still append global shortcuts.
// The Outliner itself reads the registered strategy directly.
func getGlobalHelpItems() []HelpItem {
	descriptors := make(map[OperationID]OperationDescriptor)
	for _, descriptor := range coreInputOperationDescriptors() {
		descriptors[descriptor.ID] = descriptor
	}
	strategy := SimplifiedInputStrategy()
	items := make([]HelpItem, 0, len(descriptors))
	seen := make(map[OperationID]bool, len(descriptors))
	for _, binding := range strategy.Bindings {
		if seen[binding.OperationID] {
			continue
		}
		descriptor, ok := descriptors[binding.OperationID]
		if !ok {
			continue
		}
		seen[binding.OperationID] = true
		items = append(items, HelpItem{
			Key:         displayKeysForOperation(strategy, binding.OperationID),
			Description: descriptor.Label,
			Category:    operationCategoryLabel(descriptor.Category),
		})
	}
	return items
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
	sourceView     PresentationID
	viewName       string
	ready          bool
	activeTab      int      // Current tab (category) index
	categories     []string // Ordered list of categories derived from items
	tabSearchMode  bool     // True when tab search input is active
	tabSearchInput textinput.Model
}
