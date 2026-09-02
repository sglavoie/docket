package tui

// Checklist row operation IDs. Each one acts on the item the cursor names,
// addressed as an index inside the owner task the checklist row carries.
//
// They are declared beside the refusal predicate rather than beside the
// descriptors they mint because the predicate, the key strategy, and the
// registry all name them, while only the shell registers them: an operation ID
// is vocabulary the whole grammar reads, not a fact about who binds it.
const (
	operationChecklistToggleItem OperationID = "tui.checklist.toggle-item"
	operationChecklistEditItem   OperationID = "tui.checklist.edit-item"
	operationChecklistMoveDown   OperationID = "tui.checklist.move-down"
	operationChecklistMoveUp     OperationID = "tui.checklist.move-up"
	operationChecklistMoveBottom OperationID = "tui.checklist.move-bottom"
	operationChecklistMoveTop    OperationID = "tui.checklist.move-top"
	operationChecklistAddItem    OperationID = "tui.checklist.add-item"
	operationChecklistActions    OperationID = "tui.checklist.actions"
	operationChecklistDeleteItem OperationID = "tui.checklist.delete-item"
)

// operationChecklistStart is the one checklist operation a task row owns. A
// checklist row only exists once the checklist does, so without it the whole
// row grammar is unreachable until an item is minted somewhere else.
const operationChecklistStart OperationID = "tui.checklist.start"

// checklistRowGrammar is the set of operations a checklist cursor owns. It is
// what operationRefusedOnChecklistRow exempts, so the refusal rule stays one
// question — does this operation belong to the checklist row? — instead of a
// per-key branch.
var checklistRowGrammar = map[OperationID]bool{
	operationChecklistToggleItem: true,
	operationChecklistEditItem:   true,
	operationChecklistMoveDown:   true,
	operationChecklistMoveUp:     true,
	operationChecklistMoveBottom: true,
	operationChecklistMoveTop:    true,
	operationChecklistAddItem:    true,
	operationChecklistActions:    true,
	operationChecklistDeleteItem: true,
}

// checklistRowRejectionMessage is the one sentence every refused operation
// shows from a checklist cursor. It names the row kind the action needs rather
// than the target it failed to find, because the owner task was found: it is
// simply not what the user is pointing at.
const checklistRowRejectionMessage = "move the cursor to a task row to use this action"

// coreOperationsRefusedOnChecklistRow lists the core operations a checklist
// cursor turns off. Category alone cannot name them: the core family mixes
// navigation, which is how the cursor leaves the row and therefore stays, with
// operations that reach through the row to the owner task whose pointer it
// carries. Selection is refused because a selection names task rows, and Peek
// because the item has no detail panel of its own.
var coreOperationsRefusedOnChecklistRow = map[OperationID]bool{
	operationCoreCapture:         true,
	operationCoreEdit:            true,
	operationCoreEditDescription: true,
	operationCoreExternalEditor:  true,
	operationCoreBacklog:         true,
	operationCoreToday:           true,
	operationCoreNext:            true,
	operationCoreActions:         true,
	operationCoreFocusSubtree:    true,
	operationCoreFocusParent:     true,
	operationCoreZoomSection:     true,
	operationCorePeek:            true,
	operationCoreReorderDown:     true,
	operationCoreReorderUp:       true,
	operationCoreReorderBottom:   true,
	operationCoreReorderTop:      true,
	operationCoreMultiSelect:     true,
	operationCoreRangeSelect:     true,
	operationCoreSelectAll:       true,
}

// operationRefusedOnChecklistRow decides, by category rather than by key, which
// operations a checklist cursor turns off. Every task and tag operation names
// the owner task and is refused, as are the core operations listed above.
//
// It stays out of the descriptor's Applicability field so an unconditional
// descriptor keeps looking unconditional to the registry's static direct-key
// collision check; OperationDescriptor.Applicable and OperationExecutor.Execute
// read it directly instead.
func operationRefusedOnChecklistRow(descriptor OperationDescriptor) bool {
	if checklistRowGrammar[descriptor.ID] {
		return false
	}
	switch descriptor.Category {
	case OperationCategoryTask, OperationCategoryTag:
		return true
	default:
		return coreOperationsRefusedOnChecklistRow[descriptor.ID]
	}
}
