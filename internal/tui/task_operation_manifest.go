package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// taskOperationManifest is the authoritative ordered single-task action list.
// Descriptor metadata, action-sheet grouping and ordering, and bulk eligibility
// all read it, so no surface can describe the same operation differently.
func taskOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{
			ID:       operation.OperationTaskSetCheckIn,
			Label:    "Set Check-In",
			Aliases:  []string{"schedule review", "check in", "repeat review", "clear check-in"},
			Provider: operation.ValueProviderCheckIn,
			Category: taskActionCategoryPlan,
			Risk:     operation.OperationRiskMutation,
			Bulk:     true,
		},
		{
			ID:       operation.OperationTaskDelete,
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
		{ID: operation.OperationTaskAttachments, Label: "Attachments", Aliases: []string{"add attachment"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationTaskComments, Label: "Comments", Aliases: []string{"add comment"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationTaskSetState, Label: "Set State", Aliases: []string{"workflow state", "status"}, Provider: operation.ValueProviderState, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskSetPriority, Label: "Set Priority", Aliases: []string{"importance"}, Provider: operation.ValueProviderPriority, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskSetDueDate, Label: "Set Due, Defer, or Check-In", Aliases: []string{"deadline", "due date", "defer", "hide until", "check-in", "check in"}, Provider: operation.ValueProviderDate, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		// Stepping declares no Provider: it is not a value the picker could offer,
		// it is one move along the deadline line the task already sits on, and the
		// direction is the whole argument. Both are Bulk, so a selection steps in
		// one transaction with its hidden members included.
		{ID: operation.OperationTaskStepDueEarlier, Label: "Step Due Earlier", Aliases: []string{"deadline earlier", "due earlier", "pull in", "sooner"}, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskStepDueLater, Label: "Step Due Later", Aliases: []string{"deadline later", "due later", "push out", "postpone"}, Category: taskActionCategoryPlan, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskSetTags, Label: "Set Tags & Context", Aliases: []string{"labels", "context"}, Provider: operation.ValueProviderTags, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskTogglePin, Label: "Toggle Pin", Aliases: []string{"pin task", "unpin task", "keep on top"}, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskToggleReview, Label: "Toggle Review", Aliases: []string{"flag for review", "clear review", "needs a second look"}, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskToggleHidden, Label: "Toggle Hidden", Aliases: []string{"hide task", "unhide task", "out of view"}, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, Bulk: true, DirectOnly: true},
		// Not Bulk: acknowledging is a pass over candidates one at a time, and the
		// advance to the next one is half of what the key does — a selection has no
		// next. Leaving the flag off is also what refuses it under a visual
		// selection outright instead of letting it fall back to the cursor row.
		{ID: operation.OperationTaskMarkReviewed, Label: "Mark Reviewed", Aliases: []string{"reviewed", "acknowledge", "seen it", "next review candidate"}, Category: taskActionCategoryOrganize, Risk: operation.OperationRiskMutation, DirectOnly: true},
		// Not Bulk: one resume note is one task's own sentence about where it was
		// left, so there is nothing a selection of them could mean. Leaving the flag
		// off is also what refuses the operation under a visual selection outright
		// rather than letting it fall back to the cursor row.
		{ID: operation.OperationTaskSetResumeNote, Label: "Set Resume Note", Aliases: []string{"resume note", "where was I", "clear resume note"}, Provider: operation.ValueProviderResumeNote, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		// Not Bulk for the same reason: a comment is one task's own remark, and a
		// selection of tasks has nothing to say in one voice. Comments above keeps
		// the panel that reads, edits, and deletes; this only appends.
		{ID: operation.OperationTaskAddComment, Label: "Add Comment", Aliases: []string{"comment", "quick comment", "note on task"}, Provider: operation.ValueProviderComment, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		// Not Bulk: the draft it opens is one task's, and a composer holding a
		// selection would have to be a batch creation screen the product does not
		// have. Mutation risk rather than read-only even though opening the draft
		// writes nothing, because what the key offers is the creation the draft
		// leads to: an archived owner must refuse the offer at the keystroke rather
		// than at the save.
		{ID: operation.OperationTaskRepeat, Label: "Repeat Task", Aliases: []string{"repeat", "duplicate", "again", "another like this", "copy task"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationCoreEdit, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operation.OperationCoreEditDescription, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		{ID: operation.OperationCorePeek, Category: taskActionCategoryContent, Core: true, DirectOnly: true},
		// Bulk, because one clipboard write carries a whole selection: the
		// application layer assembles every selected task's text before the adapter
		// writes it once. The flag is what makes these three resolve the selection
		// rather than the cursor, so an empty visual selection refuses here exactly
		// as a mutation does instead of quietly copying the row underneath it.
		{ID: operation.OperationTaskCopyTitle, Label: "Copy Title", Aliases: []string{"yank title", "clipboard title"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskReadOnly, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskCopyTitleDescription, Label: "Copy Title and Description", Aliases: []string{"copy with description", "yank title and description"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskReadOnly, Bulk: true, DirectOnly: true},
		{ID: operation.OperationTaskCopyDetails, Label: "Copy Description", Aliases: []string{"yank description", "clipboard description"}, Category: taskActionCategoryContent, Risk: operation.OperationRiskReadOnly, Bulk: true, DirectOnly: true},
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
		{ID: operation.OperationTaskNewChild, Label: "New Child Task", Aliases: []string{"create child task", "add child", "new subtask"}, Category: taskActionCategoryRelationships, Risk: operation.OperationRiskMutation, DirectOnly: true},
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

func (m *AppModel) taskActionOperationDescriptors() []operation.OperationDescriptor {
	manifest := taskOperationDescriptorManifest()
	descriptors := make([]operation.OperationDescriptor, 0, len(manifest))
	for _, entry := range manifest {
		if entry.Core {
			continue
		}
		source, cardinality := operation.OperationTargetCurrentTask, operation.TargetCardinalityOne
		if entry.Bulk {
			source, cardinality = operation.OperationTargetSelectedTasks, operation.TargetCardinalityOneOrMore
		}
		id := entry.ID
		descriptors = append(descriptors, operation.OperationDescriptor{
			ID:                   id,
			Label:                entry.Label,
			Aliases:              entry.Aliases,
			Category:             operation.OperationCategoryTask,
			TargetSource:         source,
			TargetCardinality:    cardinality,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability: func(context operation.CommandContext) bool {
				return m.taskActionOperationApplicable(id, context)
			},
			Risk:            entry.Risk,
			ValueProviderID: entry.Provider,
		})
	}
	return descriptors
}

func (m *AppModel) taskActionOperationApplicable(id operation.OperationID, context operation.CommandContext) bool {
	if !operation.CoreOutlinerApplicability(context) {
		return false
	}
	entry, _ := taskOperationManifestEntryFor(id)
	// Direct operations describe a stable grammar, not a menu projection. They
	// stay resolvable over a cursor that names nothing so the executor can report
	// the canonical missing-target error, but a cursor that positively names a
	// non-task entity belongs to that entity's grammar and must not bind a task
	// key. A visual selection still suppresses single-target operations instead
	// of silently falling back to the cursor.
	if entry.DirectOnly {
		if context.Selection.Visual {
			return entry.Bulk
		}
		return !operation.CommandContextNamesNonTaskEntity(context)
	}
	entities := []operation.EntityContext{context.Entity}
	if context.Selection.Visual {
		selected := operation.NormalizedTaskIdentities(context.Selection.Tasks)
		if len(selected) == 0 || len(context.Selection.Entities) != len(selected) {
			return false
		}
		if !entry.Bulk {
			return false
		}
		entities = context.Selection.Entities
	}
	for _, entity := range entities {
		if entity.TaskID == "" || entity.ProjectID == "" {
			return false
		}
		if entity.Archived && entry.Risk != operation.OperationRiskReadOnly {
			return false
		}
		if !m.taskStructureAllowsOperation(id, entity) {
			return false
		}
		if !m.taskAuxiliaryAllowsOperation(id, entity) {
			return false
		}
	}
	return true
}

// deferRefusalReason disables the whole defer section when any resolved task
// cannot be deferred. One reason covers the single terminal task and the bulk
// selection alike, and it leaves the due section reachable on the same rows the
// old defer-specific applicability check used to hide outright. It reads
// app.DeferralRefusesState so the picker refuses in the same words the mutation
// would, rather than enumerating the deferrable states a second time.
func deferRefusalReason(refs []TaskProjectRef) string {
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		if refusal := app.DeferralRefusesState(ref.Task.State()); refusal != "" {
			return refusal
		}
	}
	return ""
}

// taskStructureAllowsOperation hides destination-dependent actions that the
// domain would reject: an indent with no legal parent above or an outdent at root. A target
// that no longer resolves stays applicable so the handler reports the single
// authoritative "no longer available" error instead of the row vanishing.
func (m *AppModel) taskStructureAllowsOperation(id operation.OperationID, entity operation.EntityContext) bool {
	switch id {
	case operation.OperationTaskIndent, operation.OperationTaskOutdent:
	default:
		return true
	}
	task, project := m.taskAndProjectForIdentity(taskref.TaskIdentity{
		ProjectID: entity.ProjectID,
		TaskID:    entity.TaskID,
	})
	if task == nil || project == nil {
		return true
	}
	switch id {
	case operation.OperationTaskIndent:
		above := project.GetTaskAbove(task)
		if above == nil {
			return false
		}
		allowed, _ := task.CanIndent(above)
		return allowed
	case operation.OperationTaskOutdent:
		allowed, _ := task.CanOutdent()
		return allowed
	default:
		return true
	}
}

func (m *AppModel) registerTaskActionOperations() error {
	if m.operations.Registry == nil || m.operations.Executor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	executor := m.operations.Executor
	for _, provider := range []operation.ValueProviderID{
		operation.ValueProviderState,
		operation.ValueProviderPriority,
		operation.ValueProviderDate,
		operation.ValueProviderDueDate,
		operation.ValueProviderCheckIn,
		operation.ValueProviderTags,
		operation.ValueProviderParent,
		operation.ValueProviderChild,
		operation.ValueProviderProject,
		operation.ValueProviderTaskReference,
		operation.ValueProviderResumeNote,
		operation.ValueProviderComment,
	} {
		if err := executor.RegisterValueProvider(provider, operation.PendingPickedValueProvider); err != nil {
			return err
		}
	}
	for _, descriptor := range m.taskActionOperationDescriptors() {
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler:              m.executeTaskOperation,
			// Every handler below dispatches to an application-layer coordinator
			// or the existing atomic move message, which already owns commit and
			// rollback policy.
			DelegateTransaction: true,
			// The canonical delete request prompts before it mutates, so a generic
			// executor confirmation in front of it would ask the same question twice.
			DelegateConfirmation: descriptor.ID == operation.OperationTaskDelete,
		}); err != nil {
			return err
		}
	}
	return nil
}
