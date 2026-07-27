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

func mergeHelpItems(groups ...[]HelpItem) []HelpItem {
	var items []HelpItem
	for _, group := range groups {
		items = append(items, group...)
	}
	return dedupHelpItems(items)
}

func dedupHelpItems(items []HelpItem) []HelpItem {
	type dedupKey struct{ key, desc string }
	seen := make(map[dedupKey]bool, len(items))
	result := items[:0]
	for _, item := range items {
		key := dedupKey{item.Key, item.Description}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

// getGlobalHelpItems derives the accepted root grammar directly from the
// operation descriptors and simplified input strategy. There is no parallel
// keybinding registry.
func getGlobalHelpItems() []HelpItem {
	descriptors := make(map[OperationID]OperationDescriptor)
	for _, descriptor := range coreInputOperationDescriptors() {
		descriptors[descriptor.ID] = descriptor
	}
	strategy := SimplifiedInputStrategy()
	items := make([]HelpItem, 0, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		descriptor, ok := descriptors[binding.OperationID]
		if !ok {
			continue
		}
		key := binding.DisplayKey
		if key == "" && len(binding.Keys) > 0 {
			key = binding.Keys[0]
		}
		items = append(items, HelpItem{
			Key:         key,
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
