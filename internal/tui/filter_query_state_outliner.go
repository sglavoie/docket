package tui

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
)

// The Outliner half of the filter query state: the model owns exactly one
// narrowing predicate, and these are the reads and writes that need the model to
// answer. The derivations they call — the carrier caches, the field families,
// and the clause arithmetic — are pure and live below, so what stays here is the
// part that could not.

// setFilterQuery installs the narrowing predicate and rewrites both caches from
// it. It is the only production writer of either carrier.
func (m *OutlinerModel) setFilterQuery(narrowing models.Query) {
	if m == nil {
		return
	}
	m.filterQuery = narrowing.Canonical()
	m.filter.CommonFilterState = query.CommonFilterStateFromQuery(m.filterQuery)
	m.projectFilter = query.ProjectFilterStateFromQuery(m.filterQuery)
}

// replaceFilterClauses writes one facet's fields and re-derives.
func (m *OutlinerModel) replaceFilterClauses(
	fields []models.QueryField,
	clauses []models.QueryClause,
) {
	if m == nil {
		return
	}
	m.setFilterQuery(query.WithFields(m.filterQuery, fields, clauses))
}

// adoptFilterCarriers lifts carrier-shaped state that entered from outside the
// facet keys into the query, then re-derives so the caches and the predicate
// agree again. The restored view snapshot is its one remaining caller: it freezes
// and restores the carriers, so the predicate is rebuilt from them on the way
// back in. A lens does not come through here, because a lens definition carries
// the canonical query and enters through setFilterQuery directly.
func (m *OutlinerModel) adoptFilterCarriers() {
	if m == nil {
		return
	}
	m.setFilterQuery(query.FilterQueryFromCarriers(&m.filter.CommonFilterState, &m.projectFilter))
}

// rowPredicateQuery is the half of the filter query models.Query.Admits answers.
// The population half is lifted out because a project this Outliner never loaded
// cannot be judged from a row, and the census half because its two censuses —
// the scope's open-direct-child count and the loaded population's reference
// index — live in passesHierarchyFilter and passesDependencyFilter.
func (m *OutlinerModel) rowPredicateQuery() models.Query {
	if m == nil {
		return models.Query{}
	}
	fields := append(slices.Clone(query.FilterQueryPopulationFields), query.FilterQueryCensusFields()...)
	return query.WithoutFields(m.filterQuery, fields...)
}

// admitsTerminalPopulation is the one terminal-admission rule. Every surface
// that used to hold a rule of its own reads it instead: terminal work is on
// screen when the active query names it, and nothing else puts it there. The
// facet half is answered by the canonical filter query and the sigil half by the
// compiled narrowing gate, because "$done" typed into a search and Done chosen
// from the fs picker are the same clause said twice.
//
// Recent is the one projection that does not ask. It reports what happened, so
// it owns terminal and deferred membership outright rather than waiting for a
// query to open them.
func (m *OutlinerModel) admitsTerminalPopulation() bool {
	return m != nil &&
		(m.filterQuery.OpensTerminalPopulation() || m.activeNarrowingGate().AllowsTerminal())
}

// admitsDeferredPopulation is that same rule read on the deferral axis.
func (m *OutlinerModel) admitsDeferredPopulation() bool {
	return m != nil &&
		(m.filterQuery.OpensDeferredPopulation() || m.activeNarrowingGate().AllowsDeferred())
}

// activeNarrowingGate is the compiled query the current projection was built
// from: a lens's saved query when a lens owns the view, otherwise the search
// input's. Exactly one can be live, because slash search under a lens edits that
// lens's query rather than opening a projection of its own. A nil gate is a view
// with no narrowing query at all, and every reader here tolerates one.
func (m *OutlinerModel) activeNarrowingGate() *query.SearchPredicates {
	if m == nil {
		return nil
	}
	if m.hasActiveLens() {
		return m.lensGate
	}
	return m.search.gate
}

// filterQueryProject resolves the project a row is read inside, falling back to
// the single-project Outliner's own project the way rowIdentity does.
func (m *OutlinerModel) filterQueryProject(project *models.Project) *models.Project {
	if project != nil {
		return project
	}
	if m != nil && m.project != nil {
		return m.project
	}
	return query.EmptyFilterQueryProject
}
