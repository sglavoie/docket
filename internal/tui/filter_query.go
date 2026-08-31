package tui

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// The Outliner holds exactly one narrowing predicate: filterQuery, a
// models.Query. CommonFilterState and ProjectFilterState are caches derived from
// it, not authored state — every f and F facet writes clauses and the caches are
// rewritten from the result, so a picker, a chip, a count, and the row predicate
// can never be reading four different answers.
//
// Two halves of the one query are kept apart by field, not by struct:
//
//   - the row half is what models.Query.Admits answers about one task;
//   - the population half decides which projects load at all, which is why its
//     fields carry the reload obligation models.QueryFieldRequiresReload reports.
//
// The hierarchy fields sit in neither: they are spelled as clauses so the axis
// has one vocabulary, but they are answered by passesHierarchyFilter against the
// scope's open-direct-child census rather than by Admits against the Children a
// promotion pass may have rewritten. Evaluating them here would make "parent"
// mean one thing in All and another in a flat mode.

// filterQueryRowFields are the clause fields the f chord owns and Admits
// answers.
var filterQueryRowFields = []models.QueryField{
	models.QueryFieldPriority,
	models.QueryFieldTag,
	models.QueryFieldContext,
	models.QueryFieldDue,
	models.QueryFieldState,
	models.QueryFieldPinned,
	models.QueryFieldStale,
	models.QueryFieldDeferred,
	models.QueryFieldCompleted,
}

// filterQueryHierarchyFields are the clause fields the fh facet owns. They are
// lifted out of the Admits half by rowPredicateQuery.
var filterQueryHierarchyFields = []models.QueryField{
	models.QueryFieldParent,
	models.QueryFieldChild,
	models.QueryFieldLeaf,
}

// filterQueryPopulationFields are the clause fields the F chord owns. Every one
// of them reports true from models.QueryFieldRequiresReload.
var filterQueryPopulationFields = []models.QueryField{
	models.QueryFieldProjectTag,
	models.QueryFieldParked,
	models.QueryFieldArchived,
}

// filterQueryTaskFields is everything the f chord's Clear Filters drops: the row
// half plus the hierarchy axis, and never the population half, because clearing
// one family must not clear the other.
func filterQueryTaskFields() []models.QueryField {
	return append(slices.Clone(filterQueryRowFields), filterQueryHierarchyFields...)
}

// emptyFilterQueryProject stands in for a row whose project could not be
// resolved. models.Query.Admits refuses a nil project, and the row half of the
// filter query reads no project field, so a placeholder answers the same
// question a resolved project would without inventing a population fact.
var emptyFilterQueryProject = &models.Project{}

// queryClausesForField returns the clauses of one field, in canonical order.
func queryClausesForField(query models.Query, field models.QueryField) []models.QueryClause {
	clauses := make([]models.QueryClause, 0, 2)
	for _, clause := range query.Canonical().Clauses {
		if clause.Field == field {
			clauses = append(clauses, clause)
		}
	}
	return clauses
}

// queryWithoutFields drops every clause naming one of the given fields.
func queryWithoutFields(query models.Query, fields ...models.QueryField) models.Query {
	kept := make([]models.QueryClause, 0, len(query.Clauses))
	for _, clause := range query.Clauses {
		if slices.Contains(fields, clause.Field) {
			continue
		}
		kept = append(kept, clause)
	}
	result := query
	result.Clauses = kept
	return result.Canonical()
}

// queryWithFields replaces every clause naming one of the given fields with the
// replacements, leaving every other field alone. It is the one write a facet
// makes: a facet owns its fields outright and nothing else.
func queryWithFields(
	query models.Query,
	fields []models.QueryField,
	replacements []models.QueryClause,
) models.Query {
	result := queryWithoutFields(query, fields...)
	result.Clauses = append(result.Clauses, replacements...)
	return result.Canonical()
}

// newFilterClause mints one clause, dropping a value the vocabulary cannot
// carry. A carrier holding an unmintable value is a defect upstream of here, and
// admitting it would put a clause into the query that Admits could not answer.
func newFilterClause(
	field models.QueryField,
	polarity models.QueryPolarity,
	operator models.QueryOperator,
	value string,
) (models.QueryClause, bool) {
	clause, err := models.NewQueryClause(field, polarity, operator, value)
	if err != nil {
		return models.QueryClause{}, false
	}
	return clause, true
}

// appendFilterClause appends one positive clause. The facets select rather than
// subtract — every one of them names what to keep — so the one negative a
// carrier can express, hierarchy root, goes through newFilterClause directly
// instead of widening this helper for a single caller.
func appendFilterClause(
	clauses []models.QueryClause,
	field models.QueryField,
	operator models.QueryOperator,
	value string,
) []models.QueryClause {
	if clause, ok := newFilterClause(
		field, models.QueryPolarityPositive, operator, value); ok {
		return append(clauses, clause)
	}
	return clauses
}

// commonFilterClauses lifts the row cache back into clauses. It is the inverse
// of commonFilterStateFromQuery and exists for the two inputs that are still
// carrier-shaped: a persisted lens definition and a captured view snapshot.
// Session 9 rebases the lens onto the canonical string and this direction goes
// with it.
func commonFilterClauses(f *CommonFilterState) []models.QueryClause {
	if f == nil {
		return nil
	}
	clauses := make([]models.QueryClause, 0, 8)
	if f.Priority > 0 {
		clauses = appendFilterClause(clauses, models.QueryFieldPriority,
			models.QueryOperatorAtLeast, fmt.Sprintf("%d", f.Priority))
	}
	// An explicitly empty state set has no clause: the query has no
	// match-nothing term, so the axis narrows nothing rather than everything.
	for _, state := range f.States.Values() {
		clauses = appendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, string(state))
	}
	if f.hasCompletionAgeFilter() {
		if days := f.CompletionAge.days(); days > 0 {
			clauses = appendFilterClause(clauses, models.QueryFieldCompleted,
				models.QueryOperatorAtLeast, fmt.Sprintf("%dd", days))
		} else {
			clauses = appendFilterClause(clauses, models.QueryFieldCompleted,
				models.QueryOperatorPresent, "")
		}
	}
	for _, tag := range slices.Sorted(maps.Keys(f.Tags)) {
		clauses = appendFilterClause(clauses, models.QueryFieldTag,
			models.QueryOperatorIs, tag)
	}
	for _, name := range slices.Sorted(maps.Keys(f.Contexts)) {
		clauses = appendFilterClause(clauses, models.QueryFieldContext,
			models.QueryOperatorIs, name)
	}
	if f.NoContext {
		clauses = appendFilterClause(clauses, models.QueryFieldContext,
			models.QueryOperatorNone, "")
	}
	clauses = append(clauses, dueFilterClauses(f.DueDate)...)
	clauses = append(clauses, hierarchyFilterClauses(f.Hierarchy)...)
	if f.Stale {
		clauses = appendFilterClause(clauses, models.QueryFieldStale,
			models.QueryOperatorPresent, "")
	}
	if f.Deferred {
		clauses = appendFilterClause(clauses, models.QueryFieldDeferred,
			models.QueryOperatorPresent, "")
	}
	if f.Pinned {
		clauses = appendFilterClause(clauses, models.QueryFieldPinned,
			models.QueryOperatorPresent, "")
	}
	return clauses
}

// dueFilterClauses spells one due bucket. DueDateNone is the unset reading, so
// it takes the none operator rather than a bucket value of its own.
func dueFilterClauses(due DueDateFilter) []models.QueryClause {
	switch due {
	case DueDateOverdue:
		return filterClauseList(models.QueryFieldDue, models.QueryOperatorIs, models.QueryDueOverdue)
	case DueDateToday:
		return filterClauseList(models.QueryFieldDue, models.QueryOperatorIs, models.QueryDueToday)
	case DueDateThisWeek:
		return filterClauseList(models.QueryFieldDue, models.QueryOperatorIs, models.QueryDueWeek)
	case DueDateNone:
		return filterClauseList(models.QueryFieldDue, models.QueryOperatorNone, "")
	default:
		return nil
	}
}

// dueBucketAdmits answers one due bucket through the canonical predicate. The
// bucket is spelled as its clause and handed to models.Query.Admits, so a due
// facet and a @due search term cannot disagree about which day is overdue.
func dueBucketAdmits(task *models.Task, bucket DueDateFilter, now time.Time) bool {
	clauses := dueFilterClauses(bucket)
	if len(clauses) == 0 {
		return true
	}
	return models.Query{Clauses: clauses}.Admits(task, emptyFilterQueryProject, now)
}

// hierarchyFilterClauses spells one hierarchy position. Root is the negation of
// child, which is why the vocabulary carries no root field.
func hierarchyFilterClauses(hierarchy HierarchyFilter) []models.QueryClause {
	switch hierarchy {
	case HierarchyParent:
		return filterClauseList(models.QueryFieldParent, models.QueryOperatorPresent, "")
	case HierarchyChild:
		return filterClauseList(models.QueryFieldChild, models.QueryOperatorPresent, "")
	case HierarchyLeaf:
		return filterClauseList(models.QueryFieldLeaf, models.QueryOperatorPresent, "")
	case HierarchyRoot:
		if clause, ok := newFilterClause(models.QueryFieldChild,
			models.QueryPolarityNegative, models.QueryOperatorPresent, ""); ok {
			return []models.QueryClause{clause}
		}
		return nil
	default:
		return nil
	}
}

func filterClauseList(
	field models.QueryField,
	operator models.QueryOperator,
	value string,
) []models.QueryClause {
	if clause, ok := newFilterClause(field, models.QueryPolarityPositive, operator, value); ok {
		return []models.QueryClause{clause}
	}
	return nil
}

// projectFilterClauses lifts the population cache back into clauses.
func projectFilterClauses(f *ProjectFilterState) []models.QueryClause {
	if f == nil {
		return nil
	}
	clauses := projectTagClauses(f)
	clauses = append(clauses, populationClauses(models.QueryFieldParked, f.ParkedMode)...)
	clauses = append(clauses, populationClauses(models.QueryFieldArchived, f.ArchivedMode)...)
	return clauses
}

// projectTagClauses spells the descriptive-tag restriction alone. The F# picker
// writes it together with the parked axis and never the archived one, so the tag
// half is a constructor of its own rather than a slice of the whole population.
func projectTagClauses(f *ProjectFilterState) []models.QueryClause {
	if f == nil {
		return nil
	}
	clauses := make([]models.QueryClause, 0, 2)
	for _, tag := range f.restrictionTags() {
		clauses = appendFilterClause(clauses, models.QueryFieldProjectTag,
			models.QueryOperatorIs, tag)
	}
	if f.Untagged {
		clauses = appendFilterClause(clauses, models.QueryFieldProjectTag,
			models.QueryOperatorNone, "")
	}
	return clauses
}

// populationClauses spells one population axis. Exclude is the base workspace
// the loader already owns, so it is the absence of a clause rather than a clause
// of its own.
func populationClauses(field models.QueryField, mode projectPopulationMode) []models.QueryClause {
	switch mode {
	case projectPopulationInclude:
		return filterClauseList(field, models.QueryOperatorIs, models.QueryPopulationInclude)
	case projectPopulationOnly:
		return filterClauseList(field, models.QueryOperatorIs, models.QueryPopulationOnly)
	default:
		return nil
	}
}

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
		case models.QueryFieldParent:
			filter.Hierarchy = HierarchyParent
		case models.QueryFieldLeaf:
			filter.Hierarchy = HierarchyLeaf
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
			filter.setStateFilter(set, true)
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
// facet keys — a persisted lens definition, a restored view snapshot — into the
// query, then re-derives so the caches and the predicate agree again. It is the
// seam session 9 removes when the lens starts persisting the canonical string.
func (m *OutlinerModel) adoptFilterCarriers() {
	if m == nil {
		return
	}
	m.setFilterQuery(filterQueryFromCarriers(&m.filter.CommonFilterState, &m.projectFilter))
}

// rowPredicateQuery is the half of the filter query models.Query.Admits answers.
// The population half is lifted out because a project this Outliner never loaded
// cannot be judged from a row, and the hierarchy half because its census lives
// in passesHierarchyFilter.
func (m *OutlinerModel) rowPredicateQuery() models.Query {
	if m == nil {
		return models.Query{}
	}
	fields := append(slices.Clone(filterQueryPopulationFields), filterQueryHierarchyFields...)
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
