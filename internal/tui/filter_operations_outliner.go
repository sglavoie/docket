package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// rowFilterClauseEdit is one clause constructor per row facet. Each reads the
// picker's value and spells what its fields now say; none of them mutates
// anything, so the whole f chord has exactly one writer — applyFilterClauseEdit
// in filter_operations.go.
//
//nolint:gocyclo // One constructor per facet, kept together so the set is auditable.
func (m *OutlinerModel) rowFilterClauseEdit(invocation operation.OperationInvocation) (filterClauseEdit, error) {
	switch invocation.Descriptor.ID {
	case operationFilterPriority:
		threshold, ok := invocation.Value.(int)
		if !ok || threshold < 0 || threshold > models.PriorityMax {
			return filterClauseEdit{}, fmt.Errorf("invalid priority threshold")
		}
		if threshold == 0 {
			return filterClauseEdit{status: "Priority filter cleared"}, nil
		}
		return filterClauseEdit{
			clauses: query.FilterClauseList(models.QueryFieldPriority,
				models.QueryOperatorAtLeast, strconv.Itoa(threshold)),
			status: "Filtering " + priorityThresholdLabel(threshold),
		}, nil
	case operationFilterDue:
		token, ok := invocation.Value.(string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid due-date filter value")
		}
		shortcut, known := query.DueFilterShortcutForToken(token)
		if !known {
			return filterClauseEdit{}, fmt.Errorf("unknown due filter %q", token)
		}
		return filterClauseEdit{
			clauses: shortcut.Clauses,
			status:  "Filtering " + token,
		}, nil
	case operationFilterCompletionAge:
		age, ok := invocation.Value.(query.CompletionAgeFilter)
		if !ok || age < query.CompletionAgeAll || age > query.CompletionAge90Days {
			return filterClauseEdit{}, fmt.Errorf("invalid completion-age filter value")
		}
		return filterClauseEdit{
			clauses: query.CompletionAgeClauses(age),
			status:  "Filtering completed: " + completionAgeFilterLabel(age),
		}, nil
	case operationFilterHierarchy:
		hierarchy, ok := invocation.Value.(query.HierarchyFilter)
		if !ok || hierarchy < query.HierarchyParent || hierarchy > query.HierarchyLeaf {
			return filterClauseEdit{}, fmt.Errorf("invalid hierarchy filter value")
		}
		return filterClauseEdit{
			clauses: query.HierarchyFilterClauses(hierarchy),
			status:  "Filtering hierarchy: " + hierarchyFilterLabel(hierarchy),
		}, nil
	case operationFilterDependency:
		token, ok := invocation.Value.(string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid dependency filter value")
		}
		shortcut, known := query.DependencyFilterShortcutForToken(token)
		if !known {
			return filterClauseEdit{}, fmt.Errorf("unknown dependency filter %q", token)
		}
		return filterClauseEdit{
			clauses: shortcut.Clauses,
			status:  "Filtering " + token,
		}, nil
	case operationFilterStates:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid state filter value")
		}
		clauses := make([]models.QueryClause, 0, len(names))
		for _, name := range names {
			if !models.WorkflowState(name).IsValid() {
				return filterClauseEdit{}, fmt.Errorf("invalid workflow state %q", name)
			}
			clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
				models.QueryOperatorIs, name)
		}
		status := fmt.Sprintf("Filtering %d state(s)", len(names))
		if len(names) == 0 {
			status = "State filter cleared"
		}
		return filterClauseEdit{clauses: clauses, status: status}, nil
	case operationFilterTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid tag filter value")
		}
		clauses := make([]models.QueryClause, 0, len(names))
		for _, name := range names {
			clauses = query.AppendFilterClause(clauses, models.QueryFieldTag,
				models.QueryOperatorIs, name)
		}
		status := fmt.Sprintf("Filtering %d tag(s)", len(names))
		if len(names) == 0 {
			status = "Tag filter cleared"
		}
		return filterClauseEdit{clauses: clauses, status: status}, nil
	case operationFilterContexts:
		names, ok := invocation.Value.([]string)
		if !ok {
			return filterClauseEdit{}, fmt.Errorf("invalid context filter value")
		}
		clauses := query.ContextFilterClauses(names)
		status := "Context filter cleared"
		if len(clauses) > 0 {
			status = "Filtering context: " + strings.Join(query.FilterClauseParts(clauses), " ")
		}
		return filterClauseEdit{clauses: clauses, status: status}, nil
	case operationFilterReview:
		if m.filter.Review {
			return filterClauseEdit{status: "Review filter cleared"}, nil
		}
		return filterClauseEdit{
			clauses: query.FilterClauseList(models.QueryFieldReview, models.QueryOperatorPresent, ""),
			status:  "Filtering Review",
		}, nil
	case operationFilterStale:
		if m.filter.Stale {
			return filterClauseEdit{status: "Stale filter cleared"}, nil
		}
		return filterClauseEdit{
			clauses: query.FilterClauseList(models.QueryFieldStale, models.QueryOperatorPresent, ""),
			status:  "Filtering Stale",
		}, nil
	case operationFilterPinned:
		if m.filter.Pinned {
			return filterClauseEdit{status: "Pinned filter cleared"}, nil
		}
		return filterClauseEdit{
			clauses: query.FilterClauseList(models.QueryFieldPinned, models.QueryOperatorPresent, ""),
			status:  "Filtering Pinned",
		}, nil
	case operationFilterClear:
		return filterClauseEdit{status: "Filters cleared"}, nil
	default:
		return filterClauseEdit{}, fmt.Errorf("unknown filter operation %s", invocation.Descriptor.ID)
	}
}

func priorityThresholdLabel(threshold int) string {
	switch threshold {
	case models.PriorityHigh:
		return "High priority"
	case models.PriorityMedium:
		return "Medium priority or higher"
	case models.PriorityLow:
		return "Low priority or higher"
	default:
		return "priority"
	}
}

func hierarchyFilterLabel(filter query.HierarchyFilter) string {
	switch filter {
	case query.HierarchyParent:
		return "parent"
	case query.HierarchyChild:
		return "child"
	case query.HierarchyRoot:
		return "root"
	case query.HierarchyLeaf:
		return "leaf"
	default:
		return "all"
	}
}

func completionAgeFilterLabel(filter query.CompletionAgeFilter) string {
	if days := filter.Days(); days > 0 {
		return fmt.Sprintf("at least %d days ago", days)
	}
	return "all ages"
}
