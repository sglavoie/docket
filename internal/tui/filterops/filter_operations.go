package filterops

import (
	"fmt"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// filterOperationDescriptor is the shape every facet shares: no target, no
// transaction, and applicability decided by the Outliner alone.
func filterOperationDescriptor(
	id operation.OperationID,
	label string,
	provider operation.ValueProviderID,
	aliases ...string,
) operation.OperationDescriptor {
	return operation.OperationDescriptor{
		ID:                   id,
		Label:                label,
		Aliases:              aliases,
		Category:             operation.OperationCategoryFilter,
		TargetCardinality:    operation.TargetCardinalityNone,
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Applicability:        operation.CoreOutlinerApplicability,
		Risk:                 operation.OperationRiskReadOnly,
		ValueProviderID:      provider,
	}
}

func FilterOperationDescriptors() []operation.OperationDescriptor {
	descriptor := filterOperationDescriptor
	return []operation.OperationDescriptor{
		descriptor(operation.OperationFilterPriority, "Filter by Priority", operation.ValueProviderFilterPriority, "priority threshold"),
		descriptor(operation.OperationFilterStates, "Filter by State", operation.ValueProviderFilterStates, "state filter"),
		descriptor(operation.OperationFilterTags, "Filter by Tags", operation.ValueProviderFilterTags, "tag filter"),
		descriptor(operation.OperationFilterContexts, "Filter by Context", operation.ValueProviderFilterContexts, "context filter"),
		descriptor(operation.OperationFilterProjects, "Filter by Projects", operation.ValueProviderFilterProjects, "project filter"),
		descriptor(operation.OperationFilterReview, "Filter Review", "", "review flag filter", "flagged for review"),
		descriptor(operation.OperationFilterDue, "Filter by Due or Deferred", operation.ValueProviderFilterDue, "due filter", "deferred"),
		descriptor(operation.OperationFilterCompletionAge, "Filter by Completion Age", operation.ValueProviderFilterCompletionAge, "completion age", "cleanup age"),
		descriptor(operation.OperationFilterHierarchy, "Filter by Hierarchy", operation.ValueProviderFilterHierarchy, "hierarchy position", "parent or child"),
		descriptor(operation.OperationFilterDependency, "Filter by Dependency", operation.ValueProviderFilterDependency, "?blocked", "?blocking", "references"),
		descriptor(operation.OperationFilterStale, "Filter Stale", "", "stale work", "untouched work"),
		// The pinned axis has been in the row cache and in the canonical query all
		// along; only the key was missing, so this is a binding for an existing
		// facet rather than a new one.
		descriptor(operation.OperationFilterPinned, "Filter Pinned", "", "pinned work", "pinned tasks"),
		// Hidden writes a population clause, so applying it reloads rather than
		// rebuilds and it refuses under the ranked list like the F chord does.
		descriptor(operation.OperationFilterHidden, "Filter Hidden", "", "hidden tasks", "show hidden", "unhide"),
		descriptor(operation.OperationFilterClear, "Clear Filters", "", "reset filters"),
	}
}

func (c *Controller) RegisterFilterOperations(shell Shell) error {
	if shell.Registry() == nil || shell.Executor() == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	// Priority is the only preset-only facet: each f!0-3 binding carries its
	// threshold, so reaching the provider means a caller bypassed the grammar.
	if err := shell.Executor().RegisterValueProvider(operation.ValueProviderFilterPriority,
		func(operation.OperationInvocation) operation.OperationValueProviderResult {
			return operation.OperationValueProviderResult{
				Err: fmt.Errorf("this filter takes its value from the f!0-3 chord"),
			}
		}); err != nil {
		return err
	}
	for _, provider := range []operation.ValueProviderID{
		operation.ValueProviderFilterStates,
		operation.ValueProviderFilterTags,
		operation.ValueProviderFilterContexts,
		operation.ValueProviderFilterProjects,
		operation.ValueProviderFilterDue,
		operation.ValueProviderFilterCompletionAge,
		operation.ValueProviderFilterHierarchy,
		operation.ValueProviderFilterDependency,
		operation.ValueProviderFilterProjectTags,
	} {
		if err := shell.Executor().RegisterValueProvider(provider,
			func(operation.OperationInvocation) operation.OperationValueProviderResult {
				// Returning no immediate value asks the shell to open the provider's
				// picker seeded from the current filter.
				return operation.OperationValueProviderResult{}
			}); err != nil {
			return err
		}
	}
	// One closure body serves both families. Which of them refuses a search
	// projection is not declared here: it is read off the clause fields the
	// operation writes, so a key is advertised exactly when its handler will
	// accept it.
	register := func(descriptors []operation.OperationDescriptor, handler operation.OperationHandler) error {
		for _, descriptor := range descriptors {
			stale := descriptor.ID == operation.OperationFilterStale
			refusesSearchProjection := outliner.FilterOperationRequiresReload(descriptor.ID)
			descriptor.Applicability = func(ctx operation.CommandContext) bool {
				if !operation.CoreOutlinerApplicability(ctx) {
					return false
				}
				if refusesSearchProjection &&
					shell.Outliner() != nil && shell.Outliner().ShowsRankedList() {
					return false
				}
				return !stale || shell.Outliner() == nil ||
					shell.Outliner().ActiveOutlinerMode() != chrome.ModeRecent
			}
			if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
				RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
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
	if err := register(FilterOperationDescriptors(), func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
		return c.ExecuteFilterOperation(shell, invocation)
	}); err != nil {
		return err
	}
	return register(ProjectFilterOperationDescriptors(), func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
		return c.ExecuteProjectFilterOperation(shell, invocation)
	})
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
func (c *Controller) ApplyFilterClauseEdit(shell Shell, id operation.OperationID, edit outliner.FilterClauseEdit) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	reload := outliner.FilterOperationRequiresReload(id)
	if reload && out.ShowsRankedList() {
		return operation.OperationHandlerResult{Status: "Filters are locked in the ranked list · or returns to the narrowed search"}
	}
	if edit.Declined {
		return operation.OperationHandlerResult{Status: edit.Status}
	}
	out.ReplaceFilterClauses(outliner.FilterOperationFields(id), edit.Clauses)
	if reload {
		return operation.OperationHandlerResult{
			Message: operation.OperationCommandResultMsg{Cmd: c.ProjectFilterReloadCmd(shell, edit.Status)},
		}
	}
	out.Population.Invalidate()
	outliner.RebuildOutlinerPreservingCursor(out)
	return operation.OperationHandlerResult{Status: edit.Status}
}

func (c *Controller) ExecuteFilterOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Outliner is unavailable")}
	}
	edit, err := out.RowFilterClauseEdit(invocation)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	return c.ApplyFilterClauseEdit(shell, invocation.Descriptor.ID, edit)
}
