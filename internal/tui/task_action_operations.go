package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
	if !coreOutlinerApplicability(context) {
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
	case operationTaskIndent, operationTaskOutdent:
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
	case operationTaskIndent:
		above := project.GetTaskAbove(task)
		if above == nil {
			return false
		}
		allowed, _ := task.CanIndent(above)
		return allowed
	case operationTaskOutdent:
		allowed, _ := task.CanOutdent()
		return allowed
	default:
		return true
	}
}

// pendingPickedValueProvider registers a picked provider with the executor
// without supplying a value, so the shell constructs the provider's ListSurface
// picker from the frozen request instead. It is the picker counterpart of
// pendingTextValueProvider.
func pendingPickedValueProvider(OperationInvocation) OperationValueProviderResult {
	return OperationValueProviderResult{}
}

func (m *AppModel) registerTaskActionOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	executor := m.operationExecutor
	for _, provider := range []operation.ValueProviderID{
		operation.ValueProviderState,
		operation.ValueProviderPriority,
		operation.ValueProviderDate,
		operation.ValueProviderDueDate,
		operation.ValueProviderTags,
		operation.ValueProviderParent,
		operation.ValueProviderChild,
		operation.ValueProviderProject,
		operation.ValueProviderTaskReference,
	} {
		if err := executor.RegisterValueProvider(provider, pendingPickedValueProvider); err != nil {
			return err
		}
	}
	for _, descriptor := range m.taskActionOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler:              m.executeTaskOperation,
			// Every handler below dispatches to an application-layer coordinator
			// or the existing atomic move message, which already owns commit and
			// rollback policy.
			DelegateTransaction: true,
			// The canonical delete request prompts before it mutates, so a generic
			// executor confirmation in front of it would ask the same question twice.
			DelegateConfirmation: descriptor.ID == operationTaskDelete,
		}); err != nil {
			return err
		}
	}
	return nil
}
