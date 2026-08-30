package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.
func passesCommonFiltersAt(task *models.Task, filter *CommonFilterState, now time.Time) bool {
	query := queryWithoutFields(
		models.Query{Clauses: commonFilterClauses(filter)},
		filterQueryHierarchyFields...,
	)
	return query.Admits(task, emptyFilterQueryProject, now)
}

func passesCommonFilters(task *models.Task, filter *CommonFilterState) bool {
	return passesCommonFiltersAt(task, filter, time.Now())
}

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
func clearCommonFilters(f *CommonFilterState) {
	f.States = models.WorkflowStateSet{}
	f.statesSet = false
	f.Priority = 0
	f.Tags = make(map[string]bool)
	f.Contexts = make(map[string]bool)
	f.NoContext = false
	f.DueDate = DueDateAll
	f.CompletionAge = CompletionAgeAll
	f.completionAgeSet = false
	f.Hierarchy = HierarchyAll
	f.Stale = false
	f.Deferred = false
	f.Pinned = false
}
