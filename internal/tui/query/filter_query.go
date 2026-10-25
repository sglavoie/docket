package query

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/models"
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
// The census fields sit in neither: they are spelled as clauses so each axis has
// one vocabulary, but each is answered against a census this Outliner holds and
// Admits does not. The hierarchy fields go to passesHierarchyFilter against the
// scope's open-direct-child count rather than to Admits against the Children a
// promotion pass may have rewritten, since evaluating them there would make
// "parent" mean one thing in All and another in a flat mode. The dependency
// fields go to passesDependencyFilter against the loaded population's reference
// index, since dependents are computed rather than stored and no reading of one
// task can find them.

// filterQueryRowFields are the clause fields the f chord owns and Admits
// answers.
var filterQueryRowFields = []models.QueryField{
	models.QueryFieldPriority,
	models.QueryFieldTag,
	models.QueryFieldContext,
	models.QueryFieldProject,
	models.QueryFieldDue,
	models.QueryFieldState,
	models.QueryFieldPinned,
	models.QueryFieldReview,
	models.QueryFieldStale,
	models.QueryFieldDeferred,
	models.QueryFieldCompleted,
	models.QueryFieldHidden,
}

// FilterQueryHierarchyFields are the clause fields the fh facet owns. They are
// lifted out of the Admits half by rowPredicateQuery.
var FilterQueryHierarchyFields = []models.QueryField{
	models.QueryFieldParent,
	models.QueryFieldChild,
	models.QueryFieldLeaf,
}

// FilterQueryDependencyFields are the clause fields the f~ facet owns. They are
// lifted out of the Admits half for the same reason the hierarchy fields are:
// blocking is a fact about which other tasks name this one, which no reading of
// this one can establish, and its twin travels with it so one facet is answered
// in one place.
var FilterQueryDependencyFields = []models.QueryField{
	models.QueryFieldBlocked,
	models.QueryFieldBlocking,
}

// FilterQueryCensusFields is every field models.QueryFieldRequiresCensus marks,
// in canonical order. It is derived rather than listed so a field added to the
// census family cannot silently stay in the Admits half, where it would be
// answered by a predicate that does not hold the population.
func FilterQueryCensusFields() []models.QueryField {
	fields := make([]models.QueryField, 0, len(FilterQueryHierarchyFields)+len(FilterQueryDependencyFields))
	for _, field := range models.QueryFieldOrder() {
		if models.QueryFieldRequiresCensus(field) {
			fields = append(fields, field)
		}
	}
	return fields
}

// FilterQueryPopulationFields are the clause fields the F chord owns. Every one
// of them reports true from models.QueryFieldRequiresReload.
var FilterQueryPopulationFields = []models.QueryField{
	models.QueryFieldProjectTag,
	models.QueryFieldParked,
	models.QueryFieldArchived,
}

// FilterQueryTaskFields is everything the f chord's Clear Filters drops: the row
// half plus the census fields, and never the population half, because clearing
// one family must not clear the other.
func FilterQueryTaskFields() []models.QueryField {
	return append(slices.Clone(filterQueryRowFields), FilterQueryCensusFields()...)
}

// EmptyFilterQueryProject stands in for a row whose project could not be
// resolved. models.Query.Admits refuses a nil project, and the only row-half
// field that reads the project is the project facet, which its empty slug
// refuses, so a placeholder answers every other question a resolved project
// would without inventing a population fact.
var EmptyFilterQueryProject = &models.Project{}

// ClausesForField returns the clauses of one field, in canonical order.
func ClausesForField(query models.Query, field models.QueryField) []models.QueryClause {
	clauses := make([]models.QueryClause, 0, 2)
	for _, clause := range query.Canonical().Clauses {
		if clause.Field == field {
			clauses = append(clauses, clause)
		}
	}
	return clauses
}

// ClausesOfFields returns every clause naming one of the given fields, in
// canonical order. It is WithoutFields' complement: what one call drops is
// exactly what the other keeps, so a query split by field and put back together
// leaves no clause in both halves and none in neither.
func ClausesOfFields(query models.Query, fields ...models.QueryField) []models.QueryClause {
	canonical := query.Canonical()
	kept := make([]models.QueryClause, 0, len(canonical.Clauses))
	for _, clause := range canonical.Clauses {
		if slices.Contains(fields, clause.Field) {
			kept = append(kept, clause)
		}
	}
	return kept
}

// WithoutFields drops every clause naming one of the given fields.
func WithoutFields(query models.Query, fields ...models.QueryField) models.Query {
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

// WithFields replaces every clause naming one of the given fields with the
// replacements, leaving every other field alone. It is the one write a facet
// makes: a facet owns its fields outright and nothing else.
func WithFields(
	query models.Query,
	fields []models.QueryField,
	replacements []models.QueryClause,
) models.Query {
	result := WithoutFields(query, fields...)
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

// AppendFilterClause appends one positive clause. The facets select rather than
// subtract — every one of them names what to keep — so the one negative a
// carrier can express, hierarchy root, goes through newFilterClause directly
// instead of widening this helper for a single caller.
func AppendFilterClause(
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

// CommonFilterClauses lifts the row cache back into clauses. It is the inverse
// of CommonFilterStateFromQuery and has exactly one production reader left,
// CommonFilterParts, which renders the live cache as chips. Nothing authors a
// predicate through it any more: a persisted lens holds the canonical query, and
// so does a captured restoration point, precisely because this direction cannot
// spell a clause's polarity or hold both dependency directions at once. Keep it
// a rendering, not a second way in.
func CommonFilterClauses(f *CommonFilterState) []models.QueryClause {
	if f == nil {
		return nil
	}
	clauses := make([]models.QueryClause, 0, 8)
	if f.Priority > 0 {
		clauses = AppendFilterClause(clauses, models.QueryFieldPriority,
			models.QueryOperatorAtLeast, fmt.Sprintf("%d", f.Priority))
	}
	// An explicitly empty state set has no clause: the query has no
	// match-nothing term, so the axis narrows nothing rather than everything.
	for _, state := range f.States.Values() {
		clauses = AppendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, string(state))
	}
	if f.HasCompletionAgeFilter() {
		if days := f.CompletionAge.Days(); days > 0 {
			clauses = AppendFilterClause(clauses, models.QueryFieldCompleted,
				models.QueryOperatorAtLeast, fmt.Sprintf("%dd", days))
		} else {
			clauses = AppendFilterClause(clauses, models.QueryFieldCompleted,
				models.QueryOperatorPresent, "")
		}
	}
	for _, tag := range slices.Sorted(maps.Keys(f.Tags)) {
		clauses = AppendFilterClause(clauses, models.QueryFieldTag,
			models.QueryOperatorIs, tag)
	}
	for _, name := range slices.Sorted(maps.Keys(f.Contexts)) {
		clauses = AppendFilterClause(clauses, models.QueryFieldContext,
			models.QueryOperatorIs, name)
	}
	if f.NoContext {
		clauses = AppendFilterClause(clauses, models.QueryFieldContext,
			models.QueryOperatorNone, "")
	}
	clauses = append(clauses, ProjectFilterClauses(f.Projects)...)
	clauses = append(clauses, DueFilterClauses(f.DueDate)...)
	clauses = append(clauses, HierarchyFilterClauses(f.Hierarchy)...)
	clauses = append(clauses, DependencyFilterClauses(f.Dependency)...)
	if f.Stale {
		clauses = AppendFilterClause(clauses, models.QueryFieldStale,
			models.QueryOperatorPresent, "")
	}
	if f.Deferred {
		clauses = AppendFilterClause(clauses, models.QueryFieldDeferred,
			models.QueryOperatorPresent, "")
	}
	if f.Pinned {
		clauses = AppendFilterClause(clauses, models.QueryFieldPinned,
			models.QueryOperatorPresent, "")
	}
	if f.Review {
		clauses = AppendFilterClause(clauses, models.QueryFieldReview,
			models.QueryOperatorPresent, "")
	}
	clauses = append(clauses, PopulationClauses(models.QueryFieldHidden, f.HiddenMode)...)
	return clauses
}

// DueFilterClauses spells one due bucket. DueDateNone is the unset reading, so
// it takes the none operator rather than a bucket value of its own.
func DueFilterClauses(due DueDateFilter) []models.QueryClause {
	switch due {
	case DueDateOverdue:
		return FilterClauseList(models.QueryFieldDue, models.QueryOperatorIs, models.QueryDueOverdue)
	case DueDateToday:
		return FilterClauseList(models.QueryFieldDue, models.QueryOperatorIs, models.QueryDueToday)
	case DueDateThisWeek:
		return FilterClauseList(models.QueryFieldDue, models.QueryOperatorIs, models.QueryDueWeek)
	case DueDateNone:
		return FilterClauseList(models.QueryFieldDue, models.QueryOperatorNone, "")
	default:
		return nil
	}
}

// CompletionAgeClauses spells the cleanup facet. It carries the terminal-state
// predicate itself, because a completion age asked of open work would name a
// population that cannot answer it.
func CompletionAgeClauses(age CompletionAgeFilter) []models.QueryClause {
	clauses := FilterClauseList(models.QueryFieldState,
		models.QueryOperatorIs, string(models.StateDone))
	clauses = AppendFilterClause(clauses, models.QueryFieldState,
		models.QueryOperatorIs, string(models.StateCanceled))
	if days := age.Days(); days > 0 {
		return AppendFilterClause(clauses, models.QueryFieldCompleted,
			models.QueryOperatorAtLeast, fmt.Sprintf("%dd", days))
	}
	return AppendFilterClause(clauses, models.QueryFieldCompleted,
		models.QueryOperatorPresent, "")
}

// ContextFilterClauses spells one context picker result. The no-context
// pseudo-entry travels in the same list of names and becomes the none operator,
// so the facet needs no second value channel.
func ContextFilterClauses(names []string) []models.QueryClause {
	clauses := make([]models.QueryClause, 0, len(names))
	for _, name := range names {
		if name == NoContextFilterKey {
			clauses = AppendFilterClause(clauses, models.QueryFieldContext,
				models.QueryOperatorNone, "")
			continue
		}
		clauses = AppendFilterClause(clauses, models.QueryFieldContext,
			models.QueryOperatorIs, name)
	}
	return clauses
}

// ProjectFilterClauses spells one project picker result: one union clause per
// slug.
func ProjectFilterClauses(slugs []string) []models.QueryClause {
	clauses := make([]models.QueryClause, 0, len(slugs))
	for _, slug := range slugs {
		clauses = AppendFilterClause(clauses, models.QueryFieldProject,
			models.QueryOperatorIs, slug)
	}
	return clauses
}

// dueBucketAdmits answers one due bucket through the canonical predicate. The
// bucket is spelled as its clause and handed to models.Query.Admits, so a due
// facet and a @due search term cannot disagree about which day is overdue.
func dueBucketAdmits(task *models.Task, bucket DueDateFilter, now time.Time) bool {
	clauses := DueFilterClauses(bucket)
	if len(clauses) == 0 {
		return true
	}
	return models.Query{Clauses: clauses}.Admits(task, EmptyFilterQueryProject, now)
}

// HierarchyFilterClauses spells one hierarchy position. Root is the negation of
// child, which is why the vocabulary carries no root field.
func HierarchyFilterClauses(hierarchy HierarchyFilter) []models.QueryClause {
	switch hierarchy {
	case HierarchyParent:
		return FilterClauseList(models.QueryFieldParent, models.QueryOperatorPresent, "")
	case HierarchyChild:
		return FilterClauseList(models.QueryFieldChild, models.QueryOperatorPresent, "")
	case HierarchyLeaf:
		return FilterClauseList(models.QueryFieldLeaf, models.QueryOperatorPresent, "")
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

// DependencyFilterClauses spells one reference position.
func DependencyFilterClauses(dependency DependencyFilter) []models.QueryClause {
	switch dependency {
	case DependencyFilterBlocked:
		return FilterClauseList(models.QueryFieldBlocked, models.QueryOperatorPresent, "")
	case DependencyFilterBlocking:
		return FilterClauseList(models.QueryFieldBlocking, models.QueryOperatorPresent, "")
	default:
		return nil
	}
}

func FilterClauseList(
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
	clauses := ProjectTagClauses(f)
	clauses = append(clauses, PopulationClauses(models.QueryFieldParked, f.ParkedMode)...)
	clauses = append(clauses, PopulationClauses(models.QueryFieldArchived, f.ArchivedMode)...)
	return clauses
}

// ProjectTagClauses spells the descriptive-tag restriction alone. The F# picker
// writes it together with the parked axis and never the archived one, so the tag
// half is a constructor of its own rather than a slice of the whole population.
func ProjectTagClauses(f *ProjectFilterState) []models.QueryClause {
	if f == nil {
		return nil
	}
	clauses := make([]models.QueryClause, 0, 2)
	for _, tag := range f.restrictionTags() {
		clauses = AppendFilterClause(clauses, models.QueryFieldProjectTag,
			models.QueryOperatorIs, tag)
	}
	if f.Untagged {
		clauses = AppendFilterClause(clauses, models.QueryFieldProjectTag,
			models.QueryOperatorNone, "")
	}
	return clauses
}

// PopulationClauses spells one population axis. Exclude is the base workspace
// the loader already owns, so it is the absence of a clause rather than a clause
// of its own.
func PopulationClauses(field models.QueryField, mode ProjectPopulationMode) []models.QueryClause {
	switch mode {
	case ProjectPopulationInclude:
		return FilterClauseList(field, models.QueryOperatorIs, models.QueryPopulationInclude)
	case ProjectPopulationOnly:
		return FilterClauseList(field, models.QueryOperatorIs, models.QueryPopulationOnly)
	default:
		return nil
	}
}
