package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// filterOperationDescriptor is the shape every facet shares: no target, no
// transaction, and applicability decided by the Outliner alone.
func filterOperationDescriptor(
	id OperationID,
	label string,
	provider ValueProviderID,
	aliases ...string,
) OperationDescriptor {
	return OperationDescriptor{
		ID:                   id,
		Label:                label,
		Aliases:              aliases,
		Category:             OperationCategoryFilter,
		TargetCardinality:    TargetCardinalityNone,
		RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
		Applicability:        coreOutlinerApplicability,
		Risk:                 OperationRiskReadOnly,
		ValueProviderID:      provider,
	}
}

func filterOperationDescriptors() []OperationDescriptor {
	descriptor := filterOperationDescriptor
	return []OperationDescriptor{
		descriptor(operationFilterPriority, "Filter by Priority", ValueProviderFilterPriority, "priority threshold"),
		descriptor(operationFilterStates, "Filter by State", ValueProviderFilterStates, "state filter"),
		descriptor(operationFilterTags, "Filter by Tags", ValueProviderFilterTags, "tag filter"),
		descriptor(operationFilterContexts, "Filter by Context", ValueProviderFilterContexts, "context filter"),
		descriptor(operationFilterReview, "Filter Review", "", "review flag filter", "flagged for review"),
		descriptor(operationFilterDue, "Filter by Due or Deferred", ValueProviderFilterDue, "due filter", "deferred", "hidden"),
		descriptor(operationFilterCompletionAge, "Filter by Completion Age", ValueProviderFilterCompletionAge, "completion age", "cleanup age"),
		descriptor(operationFilterHierarchy, "Filter by Hierarchy", ValueProviderFilterHierarchy, "hierarchy position", "parent or child"),
		descriptor(operationFilterDependency, "Filter by Dependency", ValueProviderFilterDependency, "?blocked", "?blocking", "references"),
		descriptor(operationFilterStale, "Filter Stale", "", "stale work", "untouched work"),
		// The pinned axis has been in the row cache and in the canonical query all
		// along; only the key was missing, so this is a binding for an existing
		// facet rather than a new one.
		descriptor(operationFilterPinned, "Filter Pinned", "", "pinned work", "pinned tasks"),
		descriptor(operationFilterClear, "Clear Filters", "", "reset filters"),
	}
}

func (m *AppModel) registerFilterOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	// Priority is the only preset-only facet: each f!0-3 binding carries its
	// threshold, so reaching the provider means a caller bypassed the grammar.
	if err := m.operationExecutor.RegisterValueProvider(ValueProviderFilterPriority,
		func(OperationInvocation) OperationValueProviderResult {
			return OperationValueProviderResult{
				Err: fmt.Errorf("this filter takes its value from the f!0-3 chord"),
			}
		}); err != nil {
		return err
	}
	for _, provider := range []ValueProviderID{
		ValueProviderFilterStates,
		ValueProviderFilterTags,
		ValueProviderFilterContexts,
		ValueProviderFilterDue,
		ValueProviderFilterCompletionAge,
		ValueProviderFilterHierarchy,
		ValueProviderFilterDependency,
		ValueProviderFilterProjectTags,
	} {
		if err := m.operationExecutor.RegisterValueProvider(provider,
			func(OperationInvocation) OperationValueProviderResult {
				// Returning no immediate value asks the shell to open the provider's
				// picker seeded from the current filter.
				return OperationValueProviderResult{}
			}); err != nil {
			return err
		}
	}
	// One closure body serves both families. Which of them refuses a search
	// projection is not declared here: it is read off the clause fields the
	// operation writes, so a key is advertised exactly when its handler will
	// accept it.
	register := func(descriptors []OperationDescriptor, handler OperationHandler) error {
		for _, descriptor := range descriptors {
			stale := descriptor.ID == operationFilterStale
			refusesSearchProjection := filterOperationRequiresReload(descriptor.ID)
			descriptor.Applicability = func(ctx CommandContext) bool {
				if !coreOutlinerApplicability(ctx) {
					return false
				}
				if refusesSearchProjection &&
					m.crossOutliner != nil && m.crossOutliner.showsRankedList() {
					return false
				}
				return !stale || m.crossOutliner == nil ||
					m.crossOutliner.activeOutlinerMode() != chrome.ModeRecent
			}
			if err := m.registerOperation(descriptor, OperationBinding{
				RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
				Applicability:            descriptor.Applicability,
				Handler:                  handler,
				DelegateTargetResolution: true,
				DelegateConfirmation:     true,
				// Filters mutate session view state only, so no workspace transaction
				// is opened.
				DelegateTransaction: true,
			}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := register(filterOperationDescriptors(), m.executeFilterOperation); err != nil {
		return err
	}
	return register(projectFilterOperationDescriptors(), m.executeProjectFilterOperation)
}

// filterOperationFields names the clause fields one facet owns. A facet writes
// these fields outright and nothing else, so applying it can neither preserve a
// value it just replaced nor disturb a facet beside it.
func filterOperationFields(id OperationID) []models.QueryField {
	switch id {
	case operationFilterPriority:
		return []models.QueryField{models.QueryFieldPriority}
	case operationFilterStates:
		return []models.QueryField{models.QueryFieldState}
	case operationFilterTags:
		return []models.QueryField{models.QueryFieldTag}
	case operationFilterContexts:
		return []models.QueryField{models.QueryFieldContext}
	case operationFilterDue:
		// The due picker's deferred row and its four bucket rows are one facet:
		// choosing either clears the other, which is only true if both fields
		// belong to the same operation.
		return []models.QueryField{models.QueryFieldDue, models.QueryFieldDeferred}
	case operationFilterCompletionAge:
		// The cleanup facet installs the terminal-state predicate itself, which is
		// why commonFilterFacetCount reads the pair as one clearable facet.
		return []models.QueryField{models.QueryFieldCompleted, models.QueryFieldState}
	case operationFilterHierarchy:
		return filterQueryHierarchyFields
	case operationFilterDependency:
		// Both directions are one facet: choosing either clears the other, the
		// same way the due picker's buckets and its deferred row do.
		return filterQueryDependencyFields
	case operationFilterStale:
		return []models.QueryField{models.QueryFieldStale}
	case operationFilterPinned:
		return []models.QueryField{models.QueryFieldPinned}
	case operationFilterReview:
		return []models.QueryField{models.QueryFieldReview}
	case operationFilterClear:
		return filterQueryTaskFields()
	case operationFilterProjectTags:
		// The F# picker carries the parked row in the same list of names, so the
		// tag restriction and the parked axis move together.
		return []models.QueryField{models.QueryFieldProjectTag, models.QueryFieldParked}
	case operationFilterShowParked, operationFilterOnlyParked:
		return []models.QueryField{models.QueryFieldParked}
	case operationFilterIncludeArchived, operationFilterOnlyArchived:
		return []models.QueryField{models.QueryFieldArchived}
	case operationFilterProjectClear:
		return filterQueryPopulationFields
	default:
		return nil
	}
}

// filterOperationRequiresReload reports whether applying a facet reloads the
// population rather than rebuilding the rows. It is computed from the fields the
// operation writes rather than declared beside its handler, so a facet cannot be
// registered claiming one obligation and implemented honoring another.
func filterOperationRequiresReload(id OperationID) bool {
	for _, field := range filterOperationFields(id) {
		if models.QueryFieldRequiresReload(field) {
			return true
		}
	}
	return false
}

// filterClauseEdit is what one facet constructor produces: the clauses its own
// fields now carry, and the status the surface reports. Narrowing nothing is an
// empty clause list, not a separate clearing verb.
type filterClauseEdit struct {
	clauses []models.QueryClause
	status  string
	// declined reports a facet that changed nothing and only has something to
	// say, such as a clear with nothing to clear.
	declined bool
}

// applyFilterClauseEdit is the one path every facet mutation takes. It writes the
// operation's fields into the Outliner's single filter query and lets both
// carriers be re-derived from the result, so a picker, a chip, a count, and the
// row predicate cannot disagree about what is filtered.
//
// A population facet decides which projects load at all, so applying one needs a
// reload rather than a rebuild, and projectFilterReloadCmd lands the cursor at
// the top of a freshly loaded scope. That same obligation is why it refuses under
// a search projection: the projection is scored over the snapshot the reload
// replaces.
func (m *AppModel) applyFilterClauseEdit(id OperationID, edit filterClauseEdit) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	reload := filterOperationRequiresReload(id)
	if reload && out.showsRankedList() {
		return OperationHandlerResult{Status: "Exit search before changing filters"}
	}
	if edit.declined {
		return OperationHandlerResult{Status: edit.status}
	}
	out.replaceFilterClauses(filterOperationFields(id), edit.clauses)
	if reload {
		return OperationHandlerResult{
			Message: OperationCommandResultMsg{Cmd: m.projectFilterReloadCmd(edit.status)},
		}
	}
	out.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(out)
	return OperationHandlerResult{Status: edit.status}
}

func (m *AppModel) executeFilterOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	edit, err := out.rowFilterClauseEdit(invocation)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	return m.applyFilterClauseEdit(invocation.Descriptor.ID, edit)
}
