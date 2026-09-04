package tui

import "github.com/sglavoie/tt/internal/tui/operation"

// taskOperationManifest is the authoritative ordered single-task action list.
// Descriptor metadata, action-sheet grouping and ordering, and bulk eligibility
// all read it, so no surface can describe the same operation differently.
func taskOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{
			ID:       operationTaskDelete,
			Label:    "Delete",
			Aliases:  []string{"remove task", "destroy task", "delete subtree"},
			Category: taskActionCategoryDanger,
			Risk:     operation.OperationRiskDestructive,
			Bulk:     true,
		},
	}
}

// taskDirectOperationManifest keeps the direct grammar descriptors registered
// without making them task-menu rows. The task menu itself is exactly the one
// entry returned by taskOperationManifest.
func taskDirectOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: operationTaskAttachments, Label: "Attachments", Aliases: []string{"add attachment"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskComments, Label: "Comments", Aliases: []string{"add comment"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationTaskSetState, Label: "Set State", Aliases: []string{"workflow state", "status"}, Provider: operation.ValueProviderState, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskSetPriority, Label: "Set Priority", Aliases: []string{"importance"}, Provider: operation.ValueProviderPriority, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskSetDueDate, Label: "Set Due or Defer", Aliases: []string{"deadline", "due date", "defer", "hide until"}, Provider: operation.ValueProviderDate, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskSetTags, Label: "Set Tags & Context", Aliases: []string{"labels", "context"}, Provider: operation.ValueProviderTags, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operationTaskTogglePin, Label: "Toggle Pin", Aliases: []string{"pin task", "unpin task", "keep on top"}, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationCoreEdit, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operation.OperationCoreEditDescription, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operation.OperationCorePeek, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operationTaskCopyTitle, Label: "Copy Title", Aliases: []string{"yank title", "clipboard title"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskCopyTitleDescription, Label: "Copy Title and Description", Aliases: []string{"copy with description", "yank title and description"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationTaskCopyDetails, Label: "Copy Description", Aliases: []string{"yank description", "clipboard description"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operation.OperationTaskSetParent, Label: "Set Parent", Aliases: []string{"reparent", "move under", "detach from parent"}, Provider: operation.ValueProviderParent, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationTaskAdoptChildren, Label: "Adopt Children", Aliases: []string{"adopt", "add children", "nest tasks under this one"}, Provider: operation.ValueProviderChild, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, DirectOnly: true},
		// Bulk, because the preselection is tri-state rather than a checked list:
		// N tasks carrying N different stored sets is an objection to a replacement
		// picker, not to this one, and the tags editor already answers it the same
		// way. A row nobody agreed on stays indeterminate and is emitted by neither
		// half of the patch, so the tasks that disagree keep disagreeing.
		{ID: operation.OperationTaskReferences, Label: "Set Dependencies", Aliases: []string{"depends on", "references", "link tasks", "soft reference"}, Provider: operation.ValueProviderTaskReference, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskMoveToProject, Label: "Move to Project", Aliases: []string{"move project", "change project", "relocate task"}, Provider: operation.ValueProviderProject, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskIndent, Label: "Indent", Aliases: []string{"nest under previous task"}, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationTaskOutdent, Label: "Outdent", Aliases: []string{"move to parent level"}, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationTaskNewChild, Label: "New Child Task", Aliases: []string{"create child task", "add child", "new subtask"}, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, DirectOnly: true},
	}
}

func taskOperationDescriptorManifest() []operation.OperationManifestEntry {
	entries := append([]operation.OperationManifestEntry(nil), taskOperationManifest()...)
	return append(entries, taskDirectOperationManifest()...)
}

// taskOperationManifestIDs returns the authoritative action list a task row
// carries, in manifest order. Hosts attach it verbatim; the contextual sheet
// drops whatever the frozen target cannot do.
func taskOperationManifestEntryFor(id operation.OperationID) (operation.OperationManifestEntry, bool) {
	for _, entry := range taskOperationDescriptorManifest() {
		if entry.ID == id {
			return entry, true
		}
	}
	return operation.OperationManifestEntry{}, false
}

// taskOperationManifestRank orders manifest operations ahead of any other task
// action inside the same action-sheet category.
func taskOperationManifestRank(id operation.OperationID) int {
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
func taskOperationSupportsBulk(id operation.OperationID) bool {
	entry, ok := taskOperationManifestEntryFor(id)
	return ok && entry.Bulk
}

// outlinerOperationDescriptorManifest is every operation the Outliner grammar
// manifests, tasks and projects together. Only the one-path gate reads the
// concatenation: the two sheets keep asking their own manifest, so a project row
// can never appear in the task Space menu or the reverse.
func outlinerOperationDescriptorManifest() []operation.OperationManifestEntry {
	return append(taskOperationDescriptorManifest(), outlinerProjectOperationManifest()...)
}

// validateOutlinerOperationOnePath holds the whole Outliner surface to the
// shared one-path rule: a manifested operation is either bound directly and
// marked DirectOnly or listed in one of its menus, never both.
func validateOutlinerOperationOnePath(strategy operation.InputStrategy) error {
	return operation.ValidateOperationOnePath(outlinerOperationDescriptorManifest(), strategy)
}
