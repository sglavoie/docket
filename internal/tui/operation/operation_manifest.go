package operation

import (
	"fmt"

	"github.com/sglavoie/docket/internal/tui/presentation"
)

// OperationManifestEntry describes one ordered production action on an entity
// surface.
type OperationManifestEntry struct {
	ID       OperationID
	Label    string
	Aliases  []string
	Provider ValueProviderID
	Category presentation.ListCategoryID
	Risk     OperationRisk
	// Bulk marks the operations an existing multi-target coordinator supports.
	Bulk bool
	// Core marks entries whose descriptor registerCoreInputOperations already
	// owns. The manifest still ranks and groups them; their label, aliases and
	// risk stay on the single core descriptor so the action sheet and the
	// direct key can never describe the same operation differently.
	Core bool
	// DirectOnly marks entries the input strategy reaches faster than any list
	// can. They keep their descriptor, bulk coordinator and value provider but
	// leave the row action list, so the action sheet does not repeat a
	// key the footer already advertises.
	DirectOnly bool
}

// ValidateOperationOnePath rejects drift between a surface's direct grammar and
// its action menu before the application starts. A manifested operation is
// either bound directly and marked DirectOnly or listed in the menu, never both.
func ValidateOperationOnePath(manifest []OperationManifestEntry, strategy InputStrategy) error {
	direct := make(map[OperationID]bool, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		direct[binding.OperationID] = true
	}
	for _, entry := range manifest {
		switch {
		case direct[entry.ID] && !entry.DirectOnly:
			return fmt.Errorf(
				"%s operation %s is bound directly and listed in the menu",
				strategy.ID,
				entry.ID,
			)
		case entry.DirectOnly && !direct[entry.ID]:
			return fmt.Errorf(
				"%s operation %s is direct-only but has no direct binding",
				strategy.ID,
				entry.ID,
			)
		}
	}
	return nil
}
