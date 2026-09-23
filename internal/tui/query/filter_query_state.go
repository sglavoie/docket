package query

import (
	"github.com/sglavoie/tt/internal/models"
)

// CommonFilterStateFromQuery rebuilds the row cache from the query.
func CommonFilterStateFromQuery(query models.Query) CommonFilterState {
	filter := NewCommonFilterState()
	states := make([]models.WorkflowState, 0, 4)
	for _, clause := range query.Canonical().Clauses {
		switch clause.Field {
		case models.QueryFieldPriority:
			if clause.Operator == models.QueryOperatorAtLeast {
				filter.Priority = models.EffectivePriority(atoiOrZero(clause.Value))
			}
		case models.QueryFieldState:
			states = append(states, models.WorkflowState(clause.Value))
		case models.QueryFieldCompleted:
			filter.SetCompletionAgeFilter(completionAgeFromClause(clause))
		case models.QueryFieldTag:
			filter.Tags[clause.Value] = true
		case models.QueryFieldContext:
			if clause.Operator == models.QueryOperatorNone {
				filter.NoContext = true
				continue
			}
			filter.Contexts[clause.Value] = true
		case models.QueryFieldProject:
			if clause.Polarity == models.QueryPolarityPositive {
				filter.Projects = append(filter.Projects, clause.Value)
			}
		case models.QueryFieldDue:
			filter.DueDate = dueFilterFromClause(clause)
		case models.QueryFieldStale:
			filter.Stale = true
		case models.QueryFieldDeferred:
			filter.Deferred = true
		case models.QueryFieldPinned:
			filter.Pinned = true
		case models.QueryFieldReview:
			filter.Review = true
		case models.QueryFieldParent:
			filter.Hierarchy = HierarchyParent
		case models.QueryFieldLeaf:
			filter.Hierarchy = HierarchyLeaf
		case models.QueryFieldBlocked:
			filter.Dependency = DependencyFilterBlocked
		case models.QueryFieldBlocking:
			filter.Dependency = DependencyFilterBlocking
		case models.QueryFieldChild:
			filter.Hierarchy = HierarchyChild
			if clause.Polarity == models.QueryPolarityNegative {
				filter.Hierarchy = HierarchyRoot
			}
		default:
		}
	}
	if len(states) > 0 {
		set, err := models.NewWorkflowStateSet(states...)
		if err == nil {
			filter.SetStateFilter(set)
		}
	}
	return filter
}

func completionAgeFromClause(clause models.QueryClause) CompletionAgeFilter {
	if clause.Operator != models.QueryOperatorAtLeast {
		return CompletionAgeAll
	}
	switch clause.Value {
	case "7d":
		return CompletionAge7Days
	case "30d":
		return CompletionAge30Days
	case "90d":
		return CompletionAge90Days
	default:
		return CompletionAgeAll
	}
}

func dueFilterFromClause(clause models.QueryClause) DueDateFilter {
	if clause.Operator == models.QueryOperatorNone {
		return DueDateNone
	}
	switch clause.Value {
	case models.QueryDueOverdue:
		return DueDateOverdue
	case models.QueryDueToday:
		return DueDateToday
	case models.QueryDueWeek:
		return DueDateThisWeek
	default:
		return DueDateAll
	}
}

// ProjectFilterStateFromQuery rebuilds the population cache from the query.
func ProjectFilterStateFromQuery(query models.Query) ProjectFilterState {
	filter := NewProjectFilterState()
	for _, clause := range query.Canonical().Clauses {
		switch clause.Field {
		case models.QueryFieldProjectTag:
			if clause.Operator == models.QueryOperatorNone {
				filter.Untagged = true
				continue
			}
			filter.Tags[clause.Value] = true
		case models.QueryFieldParked:
			filter.ParkedMode = populationModeFromClause(clause)
		case models.QueryFieldArchived:
			filter.ArchivedMode = populationModeFromClause(clause)
		default:
		}
	}
	return filter
}

func populationModeFromClause(clause models.QueryClause) ProjectPopulationMode {
	if clause.Value == models.QueryPopulationOnly {
		return ProjectPopulationOnly
	}
	return ProjectPopulationInclude
}

// FilterClauseParts renders clauses as the chips chrome shows. The sigil text is
// the canonical spelling, so a chip, a saved lens, and a typed query name the
// same narrowing with the same characters.
func FilterClauseParts(clauses []models.QueryClause) []string {
	canonical := models.Query{Clauses: clauses}.Canonical()
	parts := make([]string, 0, len(canonical.Clauses))
	for _, clause := range canonical.Clauses {
		parts = append(parts, clause.String())
	}
	return parts
}

func atoiOrZero(value string) int {
	total := 0
	for _, symbol := range value {
		if symbol < '0' || symbol > '9' {
			return 0
		}
		total = total*10 + int(symbol-'0')
	}
	return total
}
