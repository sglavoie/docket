package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// Query is the one narrowing predicate a view holds. It is an ordered clause
// list plus three axes that are deliberately not clauses.
//
// A clause narrows rows. Mode, Scope, and Sort do not: Mode chooses which lanes
// a projection builds, Scope chooses which projects load, and Sort chooses the
// order rows appear in. None of the three can be answered by looking at one
// task, which is exactly what separates them from the clause list.
//
// Admissibility rule: a predicate belongs in the clause vocabulary only when
// (task, project, now) decides it. Anything needing workspace-wide resolution —
// whether a dependency target is open, whether a task belongs to the Today or
// Next planning collection, whether a project note matches free text — is not a
// clause here. Adding one means giving the evaluator a resolution context, not
// widening Admits; see QueryFieldOrder for how the vocabulary extends.
//
// Two families are spelled as clauses anyway, so the axis has one vocabulary a
// picker, a chip, a saved lens, and a typed query all share: the population
// fields, which QueryFieldRequiresReload marks, and the census fields, which
// QueryFieldRequiresCensus marks. Neither is answered here by pretending to
// knowledge Admits does not have — the holder of the population lifts them out
// and answers them against its own census.
//
// The zero Query narrows nothing: it admits every task in every project it is
// handed. The base workspace population (non-archived, not parked) is decided by
// the caller that loads projects, never by an implied clause.
type Query struct {
	// Clauses is the unordered working set. Canonical, String, Equal,
	// Fingerprint, and Admits all read it order-insensitively, so a caller may
	// append freely and canonicalize once.
	Clauses []QueryClause

	// Mode is the projection the query reopens into. An empty Mode means the
	// query does not own the axis.
	Mode QueryMode

	// Scope is the project the query narrows to, named by project ID. An empty
	// Scope means the query does not own the axis; it never means "force
	// global". Scope is not a clause: it decides which projects load.
	Scope string

	// Sort is the manual order axis. The zero value is the default order.
	Sort QuerySort
}

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
	{QueryFieldDue, QueryJoinOR, []QueryOperator{QueryOperatorIs, QueryOperatorNone}, false},
	{QueryFieldDefer, QueryJoinOR, []QueryOperator{QueryOperatorIs}, false},
	{QueryFieldState, QueryJoinOR, []QueryOperator{QueryOperatorIs}, false},
	{QueryFieldPinned, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
	{QueryFieldReview, QueryJoinOR, []QueryOperator{QueryOperatorPresent}, false},
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

// NewQuery builds a canonical query from axes and clauses.
func NewQuery(mode QueryMode, scope string, sort QuerySort, clauses ...QueryClause) (Query, error) {
	if err := mode.Validate(); err != nil {
		return Query{}, err
	}
	if err := sort.Validate(); err != nil {
		return Query{}, err
	}
	query := Query{Clauses: slices.Clone(clauses), Mode: mode, Scope: scope, Sort: sort}
	if err := query.Validate(); err != nil {
		return Query{}, err
	}
	return query.Canonical(), nil
}

// Validate reports the first clause or axis the query cannot honor.
func (q Query) Validate() error {
	if err := q.Mode.Validate(); err != nil {
		return err
	}
	if err := q.Sort.Validate(); err != nil {
		return err
	}
	for _, clause := range q.Clauses {
		if _, err := NewQueryClause(clause.Field, clause.Polarity, clause.Operator, clause.Value); err != nil {
			return err
		}
	}
	return nil
}

// IsEmpty reports whether the query narrows nothing and owns no axis.
func (q Query) IsEmpty() bool {
	return len(q.Clauses) == 0 && q.Mode == "" && q.Scope == "" && q.Sort == QuerySort{}
}

// RequiresReload reports whether any clause names a project population that has
// to be loaded before the predicate can see it. It reads the canonical form, so
// a hand-built clause that understates its own obligation is corrected before it
// is believed.
func (q Query) RequiresReload() bool {
	return slices.ContainsFunc(q.Canonical().Clauses, func(clause QueryClause) bool {
		return clause.RequiresReload
	})
}

// OpensTerminalPopulation reports whether the query itself asks for terminal
// work. It is the one rule terminal admission is decided by, so every carrier
// that used to hold a rule of its own asks here instead: done and canceled tasks
// sit outside the base population a view loads, and only a query naming them —
// a positive terminal state clause, or a positive completion-age clause — brings
// them back.
//
// Polarity is what makes this a population question rather than a narrowing one.
// A negative clause subtracts from a population that is already open; it never
// opens one, so it is not read here.
func (q Query) OpensTerminalPopulation() bool {
	return slices.ContainsFunc(q.Clauses, func(clause QueryClause) bool {
		if clause.Polarity != QueryPolarityPositive {
			return false
		}
		switch clause.Field {
		case QueryFieldState:
			return WorkflowState(clause.Value).IsTerminal()
		case QueryFieldCompleted:
			return true
		default:
			return false
		}
	})
}

// OpensDeferredPopulation is the same rule read on the deferral axis: work whose
// deferral has not yet expired is outside the base population until a positive
// deferred or defer-day clause names it.
func (q Query) OpensDeferredPopulation() bool {
	return slices.ContainsFunc(q.Clauses, func(clause QueryClause) bool {
		return clause.Polarity == QueryPolarityPositive &&
			(clause.Field == QueryFieldDeferred || clause.Field == QueryFieldDefer)
	})
}

// Canonical returns an equivalent query whose clauses are deduplicated, given
// their derived reload obligation, and sorted into canonical order: by field in
// QueryFieldOrder, then positives before negatives, then by operator rank, then
// by value.
func (q Query) Canonical() Query {
	canonical := Query{Mode: q.Mode, Scope: q.Scope, Sort: q.Sort}
	if len(q.Clauses) == 0 {
		return canonical
	}
	clauses := make([]QueryClause, 0, len(q.Clauses))
	for _, clause := range q.Clauses {
		if slices.ContainsFunc(clauses, clause.Equal) {
			continue
		}
		if spec, ok := querySpecFor(clause.Field); ok {
			clause.RequiresReload = spec.reload
		}
		clauses = append(clauses, clause)
	}
	slices.SortStableFunc(clauses, compareQueryClauses)
	canonical.Clauses = clauses
	return canonical
}

func compareQueryClauses(left, right QueryClause) int {
	if rank := queryFieldRank(left.Field) - queryFieldRank(right.Field); rank != 0 {
		return rank
	}
	if polarity := int(left.Polarity) - int(right.Polarity); polarity != 0 {
		return polarity
	}
	if operator := queryOperatorRank(left.Operator) - queryOperatorRank(right.Operator); operator != 0 {
		return operator
	}
	return strings.Compare(left.Value, right.Value)
}

func queryFieldRank(field QueryField) int {
	for index, spec := range queryFieldSpecs {
		if spec.field == field {
			return index
		}
	}
	return len(queryFieldSpecs)
}

func queryOperatorRank(operator QueryOperator) int {
	if index := slices.Index(queryOperatorOrder, operator); index >= 0 {
		return index
	}
	return len(queryOperatorOrder)
}

// Equal reports whether two queries mean the same thing. Clause order and
// duplicate clauses are not meaning, so both are canonicalized away first.
func (q Query) Equal(other Query) bool {
	left, right := q.Canonical(), other.Canonical()
	if left.Mode != right.Mode || left.Scope != right.Scope || left.Sort != right.Sort {
		return false
	}
	if len(left.Clauses) != len(right.Clauses) {
		return false
	}
	for index := range left.Clauses {
		if !left.Clauses[index].Equal(right.Clauses[index]) {
			return false
		}
	}
	return true
}

// Fingerprint is a stable digest of everything Equal compares: the canonical
// clause string and all three axes. Two queries share a fingerprint exactly when
// Equal reports them equal, so a memoization guard keyed on it can never be
// short-circuited by a change it cannot see.
func (q Query) Fingerprint() string {
	canonical := q.Canonical()
	sum := sha256.Sum256([]byte(strings.Join([]string{
		string(canonical.Mode),
		canonical.Scope,
		string(canonical.Sort.Field),
		fmt.Sprintf("%t", canonical.Sort.Ascending),
		canonical.String(),
	}, "\x1f")))
	return hex.EncodeToString(sum[:])[:16]
}
