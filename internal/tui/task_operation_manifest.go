package tui

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
	// leave the row action list, so the action sheet and Finder do not repeat a
	// key the footer already advertises.
	DirectOnly bool
}

// taskOperationManifest is the authoritative ordered single-task action list.
// Descriptor metadata, action-sheet grouping and ordering, and bulk eligibility
// all read it, so no surface can describe the same operation differently.
func taskOperationManifest() []taskOperationManifestEntry {
	return []taskOperationManifestEntry{
		{
			ID:         operationTaskSetState,
			Label:      "Set State",
			Aliases:    []string{"workflow state", "status"},
			Provider:   ValueProviderState,
			Category:   taskActionCategoryPlan,
			Risk:       OperationRiskMutation,
			Bulk:       true,
			DirectOnly: true,
		},
		{
			ID:         operationTaskSetPriority,
			Label:      "Set Priority",
			Aliases:    []string{"importance"},
			Provider:   ValueProviderPriority,
			Category:   taskActionCategoryPlan,
			Risk:       OperationRiskMutation,
			Bulk:       true,
			DirectOnly: true,
		},
		{
			ID:         operationTaskSetDueDate,
			Label:      "Set Due Date",
			Aliases:    []string{"deadline"},
			Provider:   ValueProviderDueDate,
			Category:   taskActionCategoryPlan,
			Risk:       OperationRiskMutation,
			Bulk:       true,
			DirectOnly: true,
		},
		{
			ID:       operationTaskSetSnooze,
			Label:    "Set Snooze",
			Aliases:  []string{"hide until"},
			Provider: ValueProviderSnooze,
			Category: taskActionCategoryPlan,
			Risk:     OperationRiskMutation,
			Bulk:     true,
		},
		{
			ID:       operationTaskScore,
			Label:    "Score Editor",
			Aliases:  []string{"edit score", "scoring", "set score"},
			Category: taskActionCategoryPlan,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskPark,
			Label:    "Park Task",
			Aliases:  []string{"parking note", "note where you left off", "resume note"},
			Category: taskActionCategoryPlan,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskUnpark,
			Label:    "Unpark Task",
			Aliases:  []string{"resume task", "clear parking note"},
			Category: taskActionCategoryPlan,
			Risk:     OperationRiskMutation,
		},
		{
			ID:         operationTaskSetTags,
			Label:      "Set Tags",
			Aliases:    []string{"labels"},
			Provider:   ValueProviderTags,
			Category:   taskActionCategoryOrganize,
			Risk:       OperationRiskMutation,
			Bulk:       true,
			DirectOnly: true,
		},
		{
			ID:       operationTaskMoveProject,
			Label:    "Move Project",
			Aliases:  []string{"transfer task"},
			Provider: ValueProviderProject,
			Category: taskActionCategoryOrganize,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskPin,
			Label:    "Pin Task",
			Aliases:  []string{"pin", "keep on top"},
			Category: taskActionCategoryOrganize,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskUnpin,
			Label:    "Unpin Task",
			Aliases:  []string{"unpin", "remove pin"},
			Category: taskActionCategoryOrganize,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskDim,
			Label:    "Dim Task",
			Aliases:  []string{"mute task", "fade task"},
			Category: taskActionCategoryOrganize,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskUndim,
			Label:    "Undim Task",
			Aliases:  []string{"unmute task", "unfade task"},
			Category: taskActionCategoryOrganize,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationCoreEdit,
			Category: taskActionCategoryContent,
			Core:     true,
		},
		{
			ID:       operationCoreDetail,
			Category: taskActionCategoryContent,
			Core:     true,
		},
		{
			ID:       operationTaskChecklist,
			Label:    "Task Checklist",
			Aliases:  []string{"checklist", "checklist items"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskComments,
			Label:    "Task Comments",
			Aliases:  []string{"comments", "add comment"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskQuickNote,
			Label:    "Quick Note",
			Aliases:  []string{"jot note", "quick comment"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskDecisions,
			Label:    "Task Decisions",
			Aliases:  []string{"decisions", "design decisions", "architecture decisions"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskAttachments,
			Label:    "Task Attachments",
			Aliases:  []string{"attachments", "add attachment"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskActivity,
			Label:    "Task Activity",
			Aliases:  []string{"activity log", "task history"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskReadOnly,
		},
		{
			ID:       operationTaskCopyTitle,
			Label:    "Copy Title",
			Aliases:  []string{"yank title", "clipboard title"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskReadOnly,
		},
		{
			ID:       operationTaskCopyTitleDescription,
			Label:    "Copy Title and Description",
			Aliases:  []string{"copy with description", "yank description"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskReadOnly,
		},
		{
			ID:       operationTaskCopyDetails,
			Label:    "Copy Details",
			Aliases:  []string{"yank details", "clipboard details"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskReadOnly,
		},
		{
			ID:       operationTaskCopyReference,
			Label:    "Copy Reference",
			Aliases:  []string{"yank reference", "copy task id"},
			Category: taskActionCategoryContent,
			Risk:     OperationRiskReadOnly,
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
			ID:       operationTaskIndent,
			Label:    "Indent",
			Aliases:  []string{"nest under previous task"},
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskOutdent,
			Label:    "Outdent",
			Aliases:  []string{"move to parent level"},
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskSetDependencies,
			Label:    "Set Dependencies",
			Aliases:  []string{"blocked by"},
			Provider: ValueProviderDependencies,
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskLinkedTasks,
			Label:    "Go to Linked Task",
			Aliases:  []string{"navigate linked tasks", "show linked", "linked tasks"},
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskReadOnly,
		},
		{
			ID:       operationTaskNewChild,
			Label:    "New Child Task",
			Aliases:  []string{"create child task", "add child", "new subtask"},
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskNewChildFromTemplate,
			Label:    "New Child Task from Template",
			Aliases:  []string{"create child task with template", "add child with template"},
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskNewLinkedSubtask,
			Label:    "New Linked Subtask",
			Aliases:  []string{"create linked subtask", "add linked subtask", "linked subtask"},
			Category: taskActionCategoryRelationships,
			Risk:     OperationRiskMutation,
		},
		{
			ID:       operationTaskDelete,
			Label:    "Delete Task",
			Aliases:  []string{"remove task", "destroy task", "delete subtree"},
			Category: taskActionCategoryDanger,
			Risk:     OperationRiskDestructive,
			Bulk:     true,
		},
	}
}

// taskOperationManifestIDs returns the authoritative action list a task row
// carries, in manifest order. Hosts attach it verbatim; the contextual sheet
// drops whatever the frozen target cannot do.
func taskOperationManifestIDs() []OperationID {
	manifest := taskOperationManifest()
	ids := make([]OperationID, 0, len(manifest))
	for _, entry := range manifest {
		if entry.DirectOnly {
			continue
		}
		ids = append(ids, entry.ID)
	}
	return ids
}

func taskOperationManifestEntryFor(id OperationID) (taskOperationManifestEntry, bool) {
	for _, entry := range taskOperationManifest() {
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
