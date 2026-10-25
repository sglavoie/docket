package query

import (
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.
//
// The root shell carries the same helper, because a carrier authored there is
// lifted the same way and a test helper cannot be shared across a package
// boundary.
func passesCommonFiltersAt(task *models.Task, filter *CommonFilterState, now time.Time) bool {
	predicate := WithoutFields(
		models.Query{Clauses: CommonFilterClauses(filter)},
		FilterQueryHierarchyFields...,
	)
	return predicate.Admits(task, EmptyFilterQueryProject, now)
}

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
// It resets by replacement rather than field by field: the armed markers behind
// the state and completion-age facets are the carrier's own, so a fresh carrier
// is the only spelling of "no facet at all" that cannot leave one of them set.
func clearCommonFilters(f *CommonFilterState) {
	*f = NewCommonFilterState()
}
