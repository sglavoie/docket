package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *AppModel) taskActionOperationDescriptors() []OperationDescriptor {
	manifest := taskOperationDescriptorManifest()
	descriptors := make([]OperationDescriptor, 0, len(manifest))
	for _, entry := range manifest {
		if entry.Core {
			continue
		}
		source, cardinality := OperationTargetCurrentTask, TargetCardinalityOne
		if entry.Bulk {
			source, cardinality = OperationTargetSelectedTasks, TargetCardinalityOneOrMore
		}
		id := entry.ID
		descriptors = append(descriptors, OperationDescriptor{
			ID:                   id,
			Label:                entry.Label,
			Aliases:              entry.Aliases,
			Category:             OperationCategoryTask,
			TargetSource:         source,
			TargetCardinality:    cardinality,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability: func(context CommandContext) bool {
				return m.taskActionOperationApplicable(id, context)
			},
			Risk:            entry.Risk,
			ValueProviderID: entry.Provider,
		})
	}
	return descriptors
}

func (m *AppModel) taskActionOperationApplicable(id OperationID, context CommandContext) bool {
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
		return !commandContextNamesNonTaskEntity(context)
	}
	entities := []EntityContext{context.Entity}
	if context.Selection.Visual {
		selected := normalizedTaskIdentities(context.Selection.Tasks)
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
		if entity.Archived && entry.Risk != OperationRiskReadOnly {
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
func (m *AppModel) taskStructureAllowsOperation(id OperationID, entity EntityContext) bool {
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
	for _, provider := range []ValueProviderID{
		ValueProviderState,
		ValueProviderPriority,
		ValueProviderDate,
		ValueProviderDueDate,
		ValueProviderTags,
		ValueProviderParent,
		ValueProviderChild,
		ValueProviderProject,
		ValueProviderTaskReference,
	} {
		if err := executor.RegisterValueProvider(provider, pendingPickedValueProvider); err != nil {
			return err
		}
	}
	for _, descriptor := range m.taskActionOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
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

// acceptsPresetValue reports whether a key binding may deliver value directly
// instead of opening the provider's picker. It mirrors the type assertions the
// task handlers below make, so a mistyped preset is rejected when the input
// strategy is registered rather than when the user presses the chord.
func (id ValueProviderID) acceptsPresetValue(value any) bool {
	switch id {
	case ValueProviderState:
		state, ok := value.(models.WorkflowState)
		return ok && state.IsValid()
	case ValueProviderPriority, ValueProviderFilterPriority:
		_, ok := value.(int)
		return ok
	case ValueProviderFilterDue:
		// A due preset is the token one row writes, so a binding that names a
		// spelling no row offers is rejected at registration.
		token, ok := value.(string)
		if !ok {
			return false
		}
		_, known := query.DueFilterShortcutForToken(token)
		return known
	case ValueProviderFilterDependency:
		// The f~ rows are their own canonical tokens too, so the same registration
		// check keeps a binding from naming a spelling no row offers.
		token, ok := value.(string)
		if !ok {
			return false
		}
		_, known := query.DependencyFilterShortcutForToken(token)
		return known
	case ValueProviderFilterCompletionAge:
		_, ok := value.(query.CompletionAgeFilter)
		return ok
	case ValueProviderFilterStates,
		ValueProviderFilterTags,
		ValueProviderFilterContexts,
		ValueProviderFilterProjectTags:
		_, ok := value.([]string)
		return ok
	case ValueProviderProjectTagDestination, ValueProviderProject:
		_, ok := value.(string)
		return ok
	case ValueProviderDate, ValueProviderDueDate:
		_, ok := value.(OperationDateValue)
		return ok
	case ValueProviderParent:
		_, ok := value.(OperationParentValue)
		return ok
	case ValueProviderChild:
		_, ok := value.([]string)
		return ok
	case ValueProviderTags:
		if _, ok := value.([]string); ok {
			return true
		}
		_, ok := value.(OperationTagsValue)
		return ok
	case ValueProviderNoteTitle, ValueProviderTagName:
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}
