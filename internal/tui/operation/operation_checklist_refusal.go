package operation

// Checklist row operation IDs. Each one acts on the item the cursor names,
// addressed as an index inside the owner task the checklist row carries.
// OperationChecklistAddItem is the exception: it names the owner alone, so it
// is also the one the task row answers, which is how a checklist that does not
// exist yet gets its first item.
//
// They are declared beside the refusal predicate rather than beside the
// descriptors they mint because the predicate, the key strategy, and the
// registry all name them, while only the shell registers them: an operation ID
// is vocabulary the whole grammar reads, not a fact about who binds it.
const (
	OperationChecklistToggleItem OperationID = "tui.checklist.toggle-item"
	OperationChecklistEditItem   OperationID = "tui.checklist.edit-item"
	OperationChecklistMoveDown   OperationID = "tui.checklist.move-down"
	OperationChecklistMoveUp     OperationID = "tui.checklist.move-up"
	OperationChecklistMoveBottom OperationID = "tui.checklist.move-bottom"
	OperationChecklistMoveTop    OperationID = "tui.checklist.move-top"
	OperationChecklistAddItem    OperationID = "tui.checklist.add-item"
	OperationChecklistActions    OperationID = "tui.checklist.actions"
	OperationChecklistDeleteItem OperationID = "tui.checklist.delete-item"
)

// ChecklistRowGrammar is the set of operations a checklist cursor owns. It is
// what operationRefusedOnChecklistRow exempts, so the refusal rule stays one
// question — does this operation belong to the checklist row? — instead of a
// per-key branch.
var ChecklistRowGrammar = map[OperationID]bool{
	OperationChecklistToggleItem: true,
	OperationChecklistEditItem:   true,
	OperationChecklistMoveDown:   true,
	OperationChecklistMoveUp:     true,
	OperationChecklistMoveBottom: true,
	OperationChecklistMoveTop:    true,
	OperationChecklistAddItem:    true,
	OperationChecklistActions:    true,
	OperationChecklistDeleteItem: true,
}

// ChecklistRowRejectionMessage is the one sentence every refused operation
// shows from a checklist cursor. It names the row kind the action needs rather
// than the target it failed to find, because the owner task was found: it is
// simply not what the user is pointing at.
const ChecklistRowRejectionMessage = "move the cursor to a task row to use this action"

// coreOperationsRefusedOnChecklistRow lists the core operations a checklist
// cursor turns off. Category alone cannot name them: the core family mixes
// navigation, which is how the cursor leaves the row and therefore stays, with
// operations that reach through the row to the owner task whose pointer it
// carries. Selection is refused because a selection names task rows, and Peek
// because the item has no detail panel of its own.
var coreOperationsRefusedOnChecklistRow = map[OperationID]bool{
	OperationCoreCapture:         true,
	OperationCoreEdit:            true,
	OperationCoreEditDescription: true,
	OperationCoreExternalEditor:  true,
	OperationCoreBacklog:         true,
	OperationCoreToday:           true,
	OperationCoreNext:            true,
	OperationCoreActions:         true,
	OperationCoreFocusSubtree:    true,
	OperationCoreFocusParent:     true,
	OperationCoreZoomSection:     true,
	OperationCorePeek:            true,
	OperationCoreReorderDown:     true,
	OperationCoreReorderUp:       true,
	OperationCoreReorderBottom:   true,
	OperationCoreReorderTop:      true,
	OperationCoreMultiSelect:     true,
	OperationCoreRangeSelect:     true,
	OperationCoreSelectAll:       true,
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
	if ChecklistRowGrammar[descriptor.ID] {
		return false
	}
	switch descriptor.Category {
	case OperationCategoryTask, OperationCategoryTag:
		return true
	default:
		return coreOperationsRefusedOnChecklistRow[descriptor.ID]
	}
}
