package tui

import "fmt"

// taskOperationManifestEntry describes one ordered production task action.
type taskOperationManifestEntry struct {
	ID       OperationID
	Label    string
	Aliases  []string
	Provider ValueProviderID
	Category ListCategoryID
	Risk     OperationRisk
	// Bulk marks the operations an existing multi-task coordinator supports.
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

// taskOperationManifest is the authoritative ordered single-task action list.
// Descriptor metadata, action-sheet grouping and ordering, and bulk eligibility
// all read it, so no surface can describe the same operation differently.
func taskOperationManifest() []taskOperationManifestEntry {
	return []taskOperationManifestEntry{
		{
			ID:       operationTaskMoveProject,
			Label:    "Move Project",
			Aliases:  []string{"transfer task"},
			Provider: ValueProviderProject,
			Category: taskActionCategoryOrganize,
			Risk:     OperationRiskMutation,
			Bulk:     true,
		},
		{
			ID:       operationTaskSetParent,
			Label:    "Set Parent",
			Aliases:  []string{"reparent", "clear parent"},
			Provider: ValueProviderParent,
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskChecklist,
			Label:    "Checklist",
			Aliases:  []string{"checklist", "checklist items"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskComments,
			Label:    "Comments",
			Aliases:  []string{"add comment"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskAttachments,
			Label:    "Attachments",
			Aliases:  []string{"add attachment"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskDelete,
			Label:    "Delete",
			Aliases:  []string{"remove task", "destroy task", "delete subtree"},
			Category: taskActionCategoryDanger,
			Risk:     OperationRiskDestructive,
			Bulk:     true,
		},
	}
}

// taskDirectOperationManifest keeps the direct grammar descriptors registered
// without making them task-menu rows. The task menu itself is exactly the six
// entries returned by taskOperationManifest.
func taskDirectOperationManifest() []taskOperationManifestEntry {
	return []taskOperationManifestEntry{
		{ID: operationTaskSetState, Label: "Set State", Aliases: []string{"workflow state", "status"}, Provider: ValueProviderState, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetPriority, Label: "Set Priority", Aliases: []string{"importance"}, Provider: ValueProviderPriority, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetDueDate, Label: "Set Due Date", Aliases: []string{"deadline"}, Provider: ValueProviderDueDate, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetSnooze, Label: "Set Snooze", Aliases: []string{"hide until"}, Provider: ValueProviderSnooze, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetTags, Label: "Set Tags", Aliases: []string{"labels"}, Provider: ValueProviderTags, Category: taskActionCategoryOrganize, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskTogglePin, Label: "Toggle Pin", Aliases: []string{"pin task", "unpin task", "keep on top"}, Category: taskActionCategoryOrganize, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationCoreEdit, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationCoreEditDescription, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationCorePeek, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationTaskCopyTitle, Label: "Copy Title", Aliases: []string{"yank title", "clipboard title"}, Category: taskActionCategoryContent, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskCopyTitleDescription, Label: "Copy Title and Description", Aliases: []string{"copy with description", "yank title and description"}, Category: taskActionCategoryContent, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskCopyDetails, Label: "Copy Description", Aliases: []string{"yank description", "clipboard description"}, Category: taskActionCategoryContent, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskIndent, Label: "Indent", Aliases: []string{"nest under previous task"}, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskOutdent, Label: "Outdent", Aliases: []string{"move to parent level"}, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskNewChild, Label: "New Child Task", Aliases: []string{"create child task", "add child", "new subtask"}, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
	}
}

func taskOperationDescriptorManifest() []taskOperationManifestEntry {
	entries := append([]taskOperationManifestEntry(nil), taskOperationManifest()...)
	return append(entries, taskDirectOperationManifest()...)
}

// taskOperationManifestIDs returns the authoritative action list a task row
// carries, in manifest order. Hosts attach it verbatim; the contextual sheet
// drops whatever the frozen target cannot do.
func taskOperationManifestEntryFor(id OperationID) (taskOperationManifestEntry, bool) {
	for _, entry := range taskOperationDescriptorManifest() {
		if entry.ID == id {
			return entry, true
		}
	}
	return taskOperationManifestEntry{}, false
}

// taskOperationManifestRank orders manifest operations ahead of any other task
// action inside the same action-sheet category.
func taskOperationManifestRank(id OperationID) int {
	manifest := taskOperationManifest()
	for index, entry := range manifest {
		if entry.ID == id {
			return index
		}
	}
	return len(manifest)
}

// taskOperationSupportsBulk reports whether the operation has a multi-task
// coordinator behind it.
func taskOperationSupportsBulk(id OperationID) bool {
	entry, ok := taskOperationManifestEntryFor(id)
	return ok && entry.Bulk
}

// validateTaskOperationOnePath rejects drift between the direct grammar and
// the task menu before the application starts. A manifested operation is either
// bound directly and marked DirectOnly or listed in the menu, never both.
func validateTaskOperationOnePath(strategy InputStrategy) error {
	direct := make(map[OperationID]bool, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		direct[binding.OperationID] = true
	}
	for _, entry := range taskOperationDescriptorManifest() {
		switch {
		case direct[entry.ID] && !entry.DirectOnly:
			return fmt.Errorf(
				"task operation %s is bound directly and listed in the task menu",
				entry.ID,
			)
		case entry.DirectOnly && !direct[entry.ID]:
			return fmt.Errorf(
				"task operation %s is direct-only but has no direct binding",
				entry.ID,
			)
		}
	}
	return nil
}
