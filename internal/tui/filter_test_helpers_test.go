package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
)

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.
func passesCommonFiltersAt(task *models.Task, filter *query.CommonFilterState, now time.Time) bool {
	predicate := query.WithoutFields(
		models.Query{Clauses: query.CommonFilterClauses(filter)},
		query.FilterQueryHierarchyFields...,
	)
	return predicate.Admits(task, query.EmptyFilterQueryProject, now)
}

func passesCommonFilters(task *models.Task, filter *query.CommonFilterState) bool {
	return passesCommonFiltersAt(task, filter, time.Now())
}

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
// It resets by replacement rather than field by field: the armed markers behind
// the state and completion-age facets are the carrier's own, so a fresh carrier
// is the only spelling of "no facet at all" that cannot leave one of them set.
func clearCommonFilters(f *query.CommonFilterState) {
	*f = query.NewCommonFilterState()
}
