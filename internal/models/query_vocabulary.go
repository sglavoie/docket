package models

import (
	"fmt"
	"slices"
)

// QueryPolarity is whether a clause admits or refuses what it names.
type QueryPolarity uint8

const (
	// QueryPolarityPositive admits rows the clause matches.
	QueryPolarityPositive QueryPolarity = iota
	// QueryPolarityNegative refuses rows the clause matches. Negative clauses
	// always compose as AND-of-nots, at any cardinality and on any field.
	QueryPolarityNegative
)

// QueryOperator is how a clause compares its value to the task attribute.
type QueryOperator string

const (
	// QueryOperatorIs matches the attribute exactly.
	QueryOperatorIs QueryOperator = "is"
	// QueryOperatorPrefix matches an attribute the value is a prefix of.
	QueryOperatorPrefix QueryOperator = "prefix"
	// QueryOperatorContains matches an attribute containing the value, case
	// insensitively. Free text is the only field that carries it.
	QueryOperatorContains QueryOperator = "contains"
	// QueryOperatorAtLeast is a threshold along the field's own scale: at least
	// this urgent for priority, at least this old for completion age.
	QueryOperatorAtLeast QueryOperator = "at_least"
	// QueryOperatorNone matches an unset attribute. Its value is always empty.
	QueryOperatorNone QueryOperator = "none"
	// QueryOperatorPresent matches when the field's boolean predicate holds. Its
	// value is always empty.
	QueryOperatorPresent QueryOperator = "present"
)

// QueryJoin is how the positive clauses of one field compose with each other.
type QueryJoin uint8

const (
	// QueryJoinOR composes the positive clauses of a single-valued attribute.
	// Two names of a field a task can only hold one of would otherwise be
	// unsatisfiable.
	QueryJoinOR QueryJoin = iota
	// QueryJoinAND composes the positive clauses of a multi-valued attribute,
	// where holding both names at once is exactly what the user asked for.
	QueryJoinAND
)

// QueryField identifies one clause family.
type QueryField string

const (
	// QueryFieldText is the free-text clause. Its corpus is the task-side one:
	// title, description, checklist, comments, tags, context, attachments, and
	// the owning project's name and slug. The notes tier lives outside a task and
	// is therefore not part of admission.
	QueryFieldText QueryField = "text"
	// QueryFieldDependency names one outgoing soft reference by durable locator.
	// It is multi-valued, so its positive clauses intersect.
	QueryFieldDependency QueryField = "dependency"
	// QueryFieldPriority carries both readings explicitly: Is for exactly that
	// priority, AtLeast for that priority or more urgent, None for unset.
	QueryFieldPriority QueryField = "priority"
	// QueryFieldTag is the multi-valued task tag axis, so its positive clauses
	// intersect.
	QueryFieldTag QueryField = "tag"
	// QueryFieldContext is the single-valued task context axis, so its positive
	// clauses union.
	QueryFieldContext QueryField = "context"
	// QueryFieldProject is the task's owning project, named by slug. A task
	// belongs to exactly one project, so its positive clauses union.
	QueryFieldProject QueryField = "project"
	// QueryFieldDue is the due-date axis: a relative bucket, an exact day, or a
	// forward window.
	QueryFieldDue QueryField = "due"
	// QueryFieldDefer matches the calendar day a task is deferred until.
	QueryFieldDefer QueryField = "defer"
	// QueryFieldState is the lifecycle state axis. Today and Next are planning
	// collections rather than task attributes and are deliberately absent.
	QueryFieldState QueryField = "state"
	// QueryFieldPinned matches pinned tasks; its negative matches unpinned ones.
	QueryFieldPinned QueryField = "pinned"
	// QueryFieldReview matches tasks flagged for review; its negative matches the
	// rest. Review is a stored boolean beside Pinned, not a lifecycle state and not
	// a tag, so it is read here the same way Pinned is.
	QueryFieldReview QueryField = "review"
	// QueryFieldNeedsReview is the canonical clause behind ?review. It stays
	// distinct from QueryFieldReview because =review asks only about the stored
	// flag, while ?review asks the shared derived attention question.
	QueryFieldNeedsReview QueryField = "needs_review"
	// QueryFieldStale matches open, undeferred work untouched for at least
	// StaleTaskThresholdDays local calendar days.
	QueryFieldStale QueryField = "stale"
	// QueryFieldDeferred matches tasks whose deferral has not yet expired.
	QueryFieldDeferred QueryField = "deferred"
	// QueryFieldCompleted matches tasks carrying a usable completion timestamp,
	// optionally at least a given age.
	QueryFieldCompleted QueryField = "completed"
	// QueryFieldParent matches tasks with at least one open direct child.
	QueryFieldParent QueryField = "parent"
	// QueryFieldChild matches tasks that sit under a parent. Its negative is the
	// root reading, which is why there is no separate root field.
	QueryFieldChild QueryField = "child"
	// QueryFieldLeaf matches tasks with no open direct child.
	QueryFieldLeaf QueryField = "leaf"
	// QueryFieldBlocked matches tasks holding at least one outgoing soft
	// reference. It is a structural fact about what the task stores and never a
	// readiness verdict: a soft reference gates nothing, so a blocked task is
	// perfectly workable.
	QueryFieldBlocked QueryField = "blocked"
	// QueryFieldBlocking matches tasks at least one other task names. Dependents
	// are computed over a population rather than stored on a task, so this is the
	// one clause field (task, project, now) cannot decide; the holder of the
	// population's reference index lifts it out and answers it there, exactly as
	// the hierarchy fields are lifted out and answered against the open-child
	// census.
	QueryFieldBlocking QueryField = "blocking"
	// QueryFieldProjectTag is the project population's descriptive tag axis. It
	// unions like a single-valued field because a project is admitted by
	// belonging to any named tag, and its None reading names the projects the
	// vocabulary cannot cover.
	QueryFieldProjectTag QueryField = "project_tag"
	// QueryFieldParked is the parked population axis.
	QueryFieldParked QueryField = "parked"
	// QueryFieldArchived is the archived population axis.
	QueryFieldArchived QueryField = "archived"
)

// Population axis values. Include loads a population without narrowing to it;
// Only both loads and isolates it. The absence of the clause is the base
// population, which the loader owns rather than the predicate.
const (
	QueryPopulationInclude = "include"
	QueryPopulationOnly    = "only"
)

// QueryDue bucket values. Every other due value is an exact day or a forward
// window; QueryOperatorNone covers a task with no due date at all.
const (
	QueryDueOverdue = "overdue"
	QueryDueToday   = "today"
	QueryDueWeek    = "week"
)

// StaleTaskThresholdDays is how long open, undeferred work must sit untouched
// before the stale clause admits it.
const StaleTaskThresholdDays = 90

// queryClearValue is the reserved token every field spells "unset" with, so one
// convention covers priority, context, due, and the project tag population.
const queryClearValue = "0"

// QueryClause is one narrowing term.
type QueryClause struct {
	Field    QueryField
	Polarity QueryPolarity
	Operator QueryOperator
	Value    string

	// RequiresReload marks a clause the row predicate alone cannot serve: the
	// project population it names has to be loaded before any task of it can be
	// evaluated. It is derived from Field and is rewritten by Canonical, so a
	// hand-built clause cannot lie about it.
	RequiresReload bool
}

// queryFieldSpec is one field's whole contract: how it renders, how its
// positives join, which operators it accepts, and whether it obliges a reload.
type queryFieldSpec struct {
	field     QueryField
	join      QueryJoin
	operators []QueryOperator
	reload    bool
}

// queryFieldSpecs is canonical field order. A canonical string renders its
// clauses in this order rather than input order, which is what lets two queries
// that mean the same thing compare byte-identical.
//
// Extending the vocabulary means inserting a spec where the field's meaning
// belongs. Only queries that carry the new field render differently, so an
// insertion never rewrites a persisted string that predates it.
var queryFieldSpecs = []queryFieldSpec{
	{QueryFieldText, QueryJoinOR, []QueryOperator{QueryOperatorContains}, false},
	{QueryFieldDependency, QueryJoinAND, []QueryOperator{QueryOperatorIs}, false},
	{QueryFieldPriority, QueryJoinOR, []QueryOperator{QueryOperatorIs, QueryOperatorAtLeast, QueryOperatorNone}, false},
	{QueryFieldTag, QueryJoinAND, []QueryOperator{QueryOperatorIs, QueryOperatorPrefix}, false},
	{QueryFieldContext, QueryJoinOR, []QueryOperator{QueryOperatorIs, QueryOperatorPrefix, QueryOperatorNone}, false},
	{QueryFieldProject, QueryJoinOR, []QueryOperator{QueryOperatorIs}, false},
	{QueryFieldDue, QueryJoinOR, []QueryOperator{QueryOperatorIs, QueryOperatorNone}, false},
	{QueryFieldDefer, QueryJoinOR, []QueryOperator{QueryOperatorIs}, false},
	{QueryFieldState, QueryJoinOR, []QueryOperator{QueryOperatorIs}, false},
	{QueryFieldPinned, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldReview, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldNeedsReview, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldStale, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldDeferred, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldCompleted, QueryJoinOR, []QueryOperator{QueryOperatorPresent, QueryOperatorAtLeast}, false},
	{QueryFieldParent, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldChild, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldLeaf, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldBlocked, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldBlocking, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldProjectTag, QueryJoinOR, []QueryOperator{QueryOperatorIs, QueryOperatorNone}, true},
	{QueryFieldParked, QueryJoinOR, []QueryOperator{QueryOperatorIs}, true},
	{QueryFieldArchived, QueryJoinOR, []QueryOperator{QueryOperatorIs}, true},
}

// queryOperatorOrder ranks operators for within-field canonical ordering.
var queryOperatorOrder = []QueryOperator{
	QueryOperatorIs,
	QueryOperatorPrefix,
	QueryOperatorContains,
	QueryOperatorAtLeast,
	QueryOperatorNone,
	QueryOperatorPresent,
}

// QueryFieldOrder returns the canonical field order.
func QueryFieldOrder() []QueryField {
	fields := make([]QueryField, len(queryFieldSpecs))
	for i, spec := range queryFieldSpecs {
		fields[i] = spec.field
	}
	return fields
}

func querySpecFor(field QueryField) (queryFieldSpec, bool) {
	for _, spec := range queryFieldSpecs {
		if spec.field == field {
			return spec, true
		}
	}
	return queryFieldSpec{}, false
}

// QueryFieldJoin reports how a field's positive clauses compose. It is the join
// law in executable form: OR when the underlying attribute is single-valued, AND
// when it is multi-valued. Negative clauses ignore it and always AND.
func QueryFieldJoin(field QueryField) (QueryJoin, error) {
	spec, ok := querySpecFor(field)
	if !ok {
		return QueryJoinOR, fmt.Errorf("unknown query field %q", field)
	}
	return spec.join, nil
}

// QueryFieldRequiresReload reports whether a field names a project population
// that has to be loaded before any task of it can be evaluated. It is the one
// predicate the reload obligation is computed from: a clause carries the answer
// on QueryClause.RequiresReload, and a caller holding only a field — a facet key
// deciding whether applying it rebuilds rows or reloads projects — asks here.
func QueryFieldRequiresReload(field QueryField) bool {
	spec, ok := querySpecFor(field)
	return ok && spec.reload
}

// QueryFieldRequiresCensus reports whether a field names a task's place in a
// population rather than an attribute of the task itself. Admits answers such a
// clause from whatever the task alone can support, which for the hierarchy
// fields is the stored Children a filter pipeline may have rewritten and for
// blocking is nothing at all. A caller holding the population's census must
// therefore lift these fields out of the row predicate and answer them itself.
//
// It is a list rather than a spec column because it is a statement about the
// callers, not about the vocabulary: the same clause is decidable inside the
// Outliner, which holds an open-child census and a reference index, and
// undecidable outside it.
func QueryFieldRequiresCensus(field QueryField) bool {
	switch field {
	case QueryFieldParent, QueryFieldChild, QueryFieldLeaf, QueryFieldBlocked, QueryFieldBlocking:
		return true
	default:
		return false
	}
}

// NewQueryClause validates one clause and fills in its derived reload
// obligation. A field cannot be handed an operator it does not declare, and a
// value the operator cannot carry is refused here rather than silently ignored
// at admission time.
func NewQueryClause(field QueryField, polarity QueryPolarity, operator QueryOperator, value string) (QueryClause, error) {
	spec, ok := querySpecFor(field)
	if !ok {
		return QueryClause{}, fmt.Errorf("unknown query field %q", field)
	}
	if !slices.Contains(spec.operators, operator) {
		return QueryClause{}, fmt.Errorf("query field %q does not accept operator %q", field, operator)
	}
	if polarity != QueryPolarityPositive && polarity != QueryPolarityNegative {
		return QueryClause{}, fmt.Errorf("query field %q has invalid polarity %d", field, polarity)
	}
	normalized, err := validateQueryValue(field, operator, value)
	if err != nil {
		return QueryClause{}, err
	}
	return QueryClause{
		Field:          field,
		Polarity:       polarity,
		Operator:       operator,
		Value:          normalized,
		RequiresReload: spec.reload,
	}, nil
}

// Equal reports clause identity. RequiresReload is derived and therefore not
// compared: two clauses naming the same narrowing are the same clause.
func (c QueryClause) Equal(other QueryClause) bool {
	return c.Field == other.Field &&
		c.Polarity == other.Polarity &&
		c.Operator == other.Operator &&
		c.Value == other.Value
}
