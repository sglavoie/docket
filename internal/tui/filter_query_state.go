package tui

import (
	"slices"

	"github.com/sglavoie/tt/internal/models"
)

// commonFilterStateFromQuery rebuilds the row cache from the query.
func commonFilterStateFromQuery(query models.Query) CommonFilterState {
	filter := newFilterState()
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
			filter.setCompletionAgeFilter(completionAgeFromClause(clause))
		case models.QueryFieldTag:
			filter.Tags[clause.Value] = true
		case models.QueryFieldContext:
			if clause.Operator == models.QueryOperatorNone {
				filter.NoContext = true
				continue
			}
			filter.Contexts[clause.Value] = true
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
			filter.setStateFilter(set)
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

// projectFilterStateFromQuery rebuilds the population cache from the query.
func projectFilterStateFromQuery(query models.Query) ProjectFilterState {
	filter := newProjectFilterState()
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

func populationModeFromClause(clause models.QueryClause) projectPopulationMode {
	if clause.Value == models.QueryPopulationOnly {
		return projectPopulationOnly
	}
	return projectPopulationInclude
}

// filterQueryFromCarriers is the one lift from carrier-shaped input into the
// canonical query.
func filterQueryFromCarriers(f *CommonFilterState, p *ProjectFilterState) models.Query {
	query := models.Query{Clauses: append(commonFilterClauses(f), projectFilterClauses(p)...)}
	return query.Canonical()
}

// setFilterQuery installs the narrowing predicate and rewrites both caches from
// it. It is the only production writer of either carrier.
func (m *OutlinerModel) setFilterQuery(query models.Query) {
	if m == nil {
		return
	}
	m.filterQuery = query.Canonical()
	m.filter.CommonFilterState = commonFilterStateFromQuery(m.filterQuery)
	m.projectFilter = projectFilterStateFromQuery(m.filterQuery)
}

// replaceFilterClauses writes one facet's fields and re-derives.
func (m *OutlinerModel) replaceFilterClauses(
	fields []models.QueryField,
	clauses []models.QueryClause,
) {
	if m == nil {
		return
	}
	m.setFilterQuery(queryWithFields(m.filterQuery, fields, clauses))
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
	m.setFilterQuery(filterQueryFromCarriers(&m.filter.CommonFilterState, &m.projectFilter))
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
	fields := append(slices.Clone(filterQueryPopulationFields), filterQueryCensusFields()...)
	return queryWithoutFields(m.filterQuery, fields...)
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
		(m.filterQuery.OpensTerminalPopulation() || m.activeNarrowingGate().allowsTerminal())
}

// admitsDeferredPopulation is that same rule read on the deferral axis.
func (m *OutlinerModel) admitsDeferredPopulation() bool {
	return m != nil &&
		(m.filterQuery.OpensDeferredPopulation() || m.activeNarrowingGate().allowsDeferred())
}

// activeNarrowingGate is the compiled query the current projection was built
// from: a lens's saved query when a lens owns the view, otherwise the search
// input's. Exactly one can be live, because slash search under a lens edits that
// lens's query rather than opening a projection of its own. A nil gate is a view
// with no narrowing query at all, and every reader here tolerates one.
func (m *OutlinerModel) activeNarrowingGate() *searchPredicates {
	if m == nil {
		return nil
	}
	if m.hasActiveLens() {
		return m.lensGate
	}
	return m.searchGate
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
	return emptyFilterQueryProject
}

// filterClauseParts renders clauses as the chips chrome shows. The sigil text is
// the canonical spelling, so a chip, a saved lens, and a typed query name the
// same narrowing with the same characters.
func filterClauseParts(clauses []models.QueryClause) []string {
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
