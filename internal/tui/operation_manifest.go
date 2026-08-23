package tui

import "fmt"

// operationManifestEntry describes one ordered production action on an entity
// surface.
type operationManifestEntry struct {
	ID       OperationID
	Label    string
	Aliases  []string
	Provider ValueProviderID
	Category ListCategoryID
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

// surfaceOperationGate names one entity surface and, once the surface manifests
// its operations, the startup check that holds it to the one-path rule.
type surfaceOperationGate struct {
	strategy InputStrategy
	validate func(InputStrategy) error
}

// surfaceOperationGates is the inventory of entity surfaces the one-operation
// grammar governs.
func surfaceOperationGates() []surfaceOperationGate {
	return []surfaceOperationGate{
		{strategy: SimplifiedInputStrategy(), validate: validateOutlinerOperationOnePath},
		{strategy: ProjectHubInputStrategy(), validate: validateProjectOperationOnePath},
		{strategy: NotesInputStrategy(), validate: validateNotesOperationOnePath},
		{strategy: AttachmentsInputStrategy(), validate: validateAttachmentOperationOnePath},
		{strategy: CommentsInputStrategy(), validate: validateCommentOperationOnePath},
	}
}

// validateOperationOnePath rejects drift between a surface's direct grammar and
// its action menu before the application starts. A manifested operation is
// either bound directly and marked DirectOnly or listed in the menu, never both.
func validateOperationOnePath(manifest []operationManifestEntry, strategy InputStrategy) error {
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
