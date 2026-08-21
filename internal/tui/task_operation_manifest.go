package tui

// taskOperationManifest is the authoritative ordered single-task action list.
// Descriptor metadata, action-sheet grouping and ordering, and bulk eligibility
// all read it, so no surface can describe the same operation differently.
func taskOperationManifest() []operationManifestEntry {
	return []operationManifestEntry{
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
// without making them task-menu rows. The task menu itself is exactly the three
// entries returned by taskOperationManifest.
func taskDirectOperationManifest() []operationManifestEntry {
	return []operationManifestEntry{
		{ID: operationTaskSetState, Label: "Set State", Aliases: []string{"workflow state", "status"}, Provider: ValueProviderState, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetPriority, Label: "Set Priority", Aliases: []string{"importance"}, Provider: ValueProviderPriority, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetDueDate, Label: "Set Date", Aliases: []string{"deadline", "due date", "defer", "hide until"}, Provider: ValueProviderDate, Category: taskActionCategoryPlan, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskSetTags, Label: "Set Tags & Context", Aliases: []string{"labels", "context"}, Provider: ValueProviderTags, Category: taskActionCategoryOrganize, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskTogglePin, Label: "Toggle Pin", Aliases: []string{"pin task", "unpin task", "keep on top"}, Category: taskActionCategoryOrganize, Risk: OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationCoreEdit, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationCoreEditDescription, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationCorePeek, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationTaskCopyTitle, Label: "Copy Title", Aliases: []string{"yank title", "clipboard title"}, Category: taskActionCategoryContent, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskCopyTitleDescription, Label: "Copy Title and Description", Aliases: []string{"copy with description", "yank title and description"}, Category: taskActionCategoryContent, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskCopyDetails, Label: "Copy Description", Aliases: []string{"yank description", "clipboard description"}, Category: taskActionCategoryContent, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskSetParent, Label: "Set Parent", Aliases: []string{"reparent", "move under", "detach from parent"}, Provider: ValueProviderParent, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskAdoptChildren, Label: "Adopt Children", Aliases: []string{"adopt", "add children", "nest tasks under this one"}, Provider: ValueProviderChild, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskMoveToProject, Label: "Move to Project", Aliases: []string{"move project", "change project", "relocate task"}, Provider: ValueProviderProject, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskIndent, Label: "Indent", Aliases: []string{"nest under previous task"}, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskOutdent, Label: "Outdent", Aliases: []string{"move to parent level"}, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskNewChild, Label: "New Child Task", Aliases: []string{"create child task", "add child", "new subtask"}, Category: taskActionCategoryRelationships, Risk: OperationRiskMutation, DirectOnly: true},
	}
}

func taskOperationDescriptorManifest() []operationManifestEntry {
	entries := append([]operationManifestEntry(nil), taskOperationManifest()...)
	return append(entries, taskDirectOperationManifest()...)
}

// taskOperationManifestIDs returns the authoritative action list a task row
// carries, in manifest order. Hosts attach it verbatim; the contextual sheet
// drops whatever the frozen target cannot do.
func taskOperationManifestEntryFor(id OperationID) (operationManifestEntry, bool) {
	for _, entry := range taskOperationDescriptorManifest() {
		if entry.ID == id {
			return entry, true
		}
	}
	return operationManifestEntry{}, false
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

// validateTaskOperationOnePath holds the task surface to the shared one-path
// rule: a manifested operation is either bound directly and marked DirectOnly
// or listed in the task menu, never both.
func validateTaskOperationOnePath(strategy InputStrategy) error {
	return validateOperationOnePath(taskOperationDescriptorManifest(), strategy)
}
