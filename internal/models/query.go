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
// deferred or defer-day clause names it. The derived review clause opens it too,
// because a stored review flag remains current attention under deferral even
// though an arrived scheduled check-in is suppressed.
func (q Query) OpensDeferredPopulation() bool {
	return slices.ContainsFunc(q.Clauses, func(clause QueryClause) bool {
		return clause.Polarity == QueryPolarityPositive &&
			(clause.Field == QueryFieldDeferred || clause.Field == QueryFieldDefer ||
				clause.Field == QueryFieldNeedsReview)
	})
}

// OpensHiddenPopulation is the same rule read on the hidden axis: an
// effectively hidden task is outside every population a reader assembles until
// a positive hidden clause names it. Include opens it beside everything else;
// only also narrows to it, which Admits applies as a row predicate.
func (q Query) OpensHiddenPopulation() bool {
	return slices.ContainsFunc(q.Clauses, func(clause QueryClause) bool {
		return clause.Polarity == QueryPolarityPositive && clause.Field == QueryFieldHidden
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
