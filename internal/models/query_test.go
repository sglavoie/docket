package models

import (
	"testing"
	"time"
)

// queryTestNow is the fixed instant every query test evaluates at, so a relative
// clause is asserted against a date the test itself states rather than the wall
// clock.
func queryTestNow() time.Time {
	return time.Date(2026, 8, 29, 10, 0, 0, 0, time.Local)
}

func mustQueryClause(t *testing.T, field QueryField, polarity QueryPolarity, operator QueryOperator, value string) QueryClause {
	t.Helper()
	clause, err := NewQueryClause(field, polarity, operator, value)
	if err != nil {
		t.Fatalf("NewQueryClause(%q, %d, %q, %q): %v", field, polarity, operator, value, err)
	}
	return clause
}

func positiveClause(t *testing.T, field QueryField, operator QueryOperator, value string) QueryClause {
	t.Helper()
	return mustQueryClause(t, field, QueryPolarityPositive, operator, value)
}

func negativeClause(t *testing.T, field QueryField, operator QueryOperator, value string) QueryClause {
	t.Helper()
	return mustQueryClause(t, field, QueryPolarityNegative, operator, value)
}

func queryTestProject() *Project {
	return &Project{ID: "project-alpha", Slug: "alpha", Name: "Alpha"}
}

func queryTestTask() *Task {
	now := queryTestNow()
	return &Task{
		ID:             "task-1",
		ShortID:        1,
		Title:          "Wire the login form",
		WorkflowState:  StateDoing,
		Priority:       PriorityMedium,
		CreatedAt:      now,
		UpdatedAt:      now,
		StateChangedAt: now,
	}
}

func TestNewQueryClauseRejectsAnOperatorTheFieldDoesNotDeclare(t *testing.T) {
	if _, err := NewQueryClause(QueryFieldPinned, QueryPolarityPositive, QueryOperatorIs, "yes"); err == nil {
		t.Fatal("pinned accepted the is operator, want a refusal")
	}
	if _, err := NewQueryClause(QueryFieldTag, QueryPolarityPositive, QueryOperatorPresent, ""); err == nil {
		t.Fatal("tag accepted the present operator, want a refusal")
	}
	if _, err := NewQueryClause("nonexistent", QueryPolarityPositive, QueryOperatorIs, "x"); err == nil {
		t.Fatal("an unknown field was accepted, want a refusal")
	}
}

func TestNewQueryClauseRejectsAValueTheFieldCannotEvaluate(t *testing.T) {
	cases := []struct {
		name     string
		field    QueryField
		operator QueryOperator
		value    string
	}{
		{"priority out of range", QueryFieldPriority, QueryOperatorIs, "9"},
		{"priority unset spelled as a value", QueryFieldPriority, QueryOperatorIs, "0"},
		{"state not in the lifecycle", QueryFieldState, QueryOperatorIs, "review"},
		{"tag not canonical", QueryFieldTag, QueryOperatorIs, "Auth Flow"},
		{"context spelled as the clear token", QueryFieldContext, QueryOperatorIs, "0"},
		{"dependency without a project", QueryFieldDependency, QueryOperatorIs, "task-9"},
		{"due window without a unit", QueryFieldDue, QueryOperatorIs, "+3"},
		{"defer bucket instead of a day", QueryFieldDefer, QueryOperatorIs, "tomorrow"},
		{"completion age in hours", QueryFieldCompleted, QueryOperatorAtLeast, "12h"},
		{"population outside the vocabulary", QueryFieldParked, QueryOperatorIs, "exclude"},
		{"flag carrying a value", QueryFieldStale, QueryOperatorPresent, "90"},
		{"empty free text", QueryFieldText, QueryOperatorContains, "   "},
		{"blank project slug", QueryFieldProject, QueryOperatorIs, " "},
		{"project slug with a space", QueryFieldProject, QueryOperatorIs, "a b"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NewQueryClause(testCase.field, QueryPolarityPositive, testCase.operator, testCase.value); err == nil {
				t.Fatalf("%s was accepted, want a refusal", testCase.name)
			}
		})
	}
}

func TestQueryFieldOrderCoversEveryFieldExactlyOnce(t *testing.T) {
	fields := QueryFieldOrder()
	seen := make(map[QueryField]bool, len(fields))
	for _, field := range fields {
		if seen[field] {
			t.Fatalf("field %q appears twice in canonical order", field)
		}
		seen[field] = true
		if _, err := QueryFieldJoin(field); err != nil {
			t.Fatalf("field %q has no join law: %v", field, err)
		}
	}
	if _, err := QueryFieldJoin("nonexistent"); err == nil {
		t.Fatal("an unknown field reported a join law, want a refusal")
	}
}

func TestQueryCanonicalOrdersClausesByFieldThenPolarityThenValue(t *testing.T) {
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldStale, QueryOperatorPresent, ""),
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "zebra"),
		negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		positiveClause(t, QueryFieldPriority, QueryOperatorAtLeast, "2"),
		positiveClause(t, QueryFieldProject, QueryOperatorIs, "beta"),
		positiveClause(t, QueryFieldProject, QueryOperatorIs, "alpha"),
	}}
	if got, want := query.Canonical().String(), "!2+ #auth #zebra -#auth %alpha %beta ?stale"; got != want {
		t.Fatalf("canonical string = %q, want %q", got, want)
	}
}

func TestQueryCanonicalDropsDuplicatesAndRederivesTheReloadObligation(t *testing.T) {
	tagged := positiveClause(t, QueryFieldProjectTag, QueryOperatorIs, "infra")
	tagged.RequiresReload = false
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		tagged,
	}}
	canonical := query.Canonical()
	if len(canonical.Clauses) != 2 {
		t.Fatalf("canonical clause count = %d, want 2", len(canonical.Clauses))
	}
	for _, clause := range canonical.Clauses {
		if clause.Field == QueryFieldProjectTag && !clause.RequiresReload {
			t.Fatal("a population clause lost its reload obligation")
		}
		if clause.Field == QueryFieldTag && clause.RequiresReload {
			t.Fatal("a task clause gained a reload obligation")
		}
	}
	if !canonical.RequiresReload() {
		t.Fatal("Query.RequiresReload = false with a population clause present")
	}
	taskOnly := Query{Clauses: []QueryClause{positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth")}}
	if taskOnly.RequiresReload() {
		t.Fatal("Query.RequiresReload = true with only task clauses present")
	}
}

func TestQueryEqualIgnoresClauseOrderAndDuplication(t *testing.T) {
	left := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		positiveClause(t, QueryFieldPriority, QueryOperatorAtLeast, "2"),
	}}
	right := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldPriority, QueryOperatorAtLeast, "2"),
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
	}}
	if !left.Equal(right) {
		t.Fatalf("%q and %q compared unequal", left.String(), right.String())
	}
	if left.Fingerprint() != right.Fingerprint() {
		t.Fatal("equal queries produced different fingerprints")
	}
}

func TestQueryEqualSeparatesTheThreeNonClauseAxes(t *testing.T) {
	base, err := NewQuery(QueryModeAll, "", QuerySort{}, positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"))
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	cases := map[string]Query{
		"mode":  {Clauses: base.Clauses, Mode: QueryModeToday},
		"scope": {Clauses: base.Clauses, Mode: QueryModeAll, Scope: "project-alpha"},
		"sort":  {Clauses: base.Clauses, Mode: QueryModeAll, Sort: QuerySort{Field: QuerySortDue, Ascending: true}},
	}
	for name, other := range cases {
		t.Run(name, func(t *testing.T) {
			if base.Equal(other) {
				t.Fatalf("queries differing only in %s compared equal", name)
			}
			if base.Fingerprint() == other.Fingerprint() {
				t.Fatalf("queries differing only in %s shared a fingerprint", name)
			}
			if base.String() != other.String() {
				t.Fatalf("the %s axis leaked into the canonical clause string", name)
			}
		})
	}
}

func TestQueryFingerprintIsStableAcrossRebuilds(t *testing.T) {
	build := func() Query {
		return Query{
			Clauses: []QueryClause{
				positiveClause(t, QueryFieldContext, QueryOperatorIs, "home"),
				negativeClause(t, QueryFieldTag, QueryOperatorPrefix, "wip"),
			},
			Mode:  QueryModeToday,
			Scope: "project-alpha",
			Sort:  QuerySort{Field: QuerySortPriority, Ascending: true},
		}
	}
	first, second := build().Fingerprint(), build().Fingerprint()
	if first != second {
		t.Fatalf("the same query fingerprinted as %q then %q", first, second)
	}
	if len(first) != 16 {
		t.Fatalf("fingerprint length = %d, want 16", len(first))
	}
}

func TestNewQueryValidatesEveryAxisAndClause(t *testing.T) {
	if _, err := NewQuery("weekly", "", QuerySort{}); err == nil {
		t.Fatal("an unknown mode was accepted")
	}
	if _, err := NewQuery(QueryModeAll, "", QuerySort{Field: "manual"}); err == nil {
		t.Fatal("an unknown sort field was accepted")
	}
	broken := Query{Clauses: []QueryClause{{Field: QueryFieldState, Operator: QueryOperatorIs, Value: "review"}}}
	if err := broken.Validate(); err == nil {
		t.Fatal("a clause naming a retired state validated")
	}
	if (Query{}).IsEmpty() != true {
		t.Fatal("the zero query is not empty")
	}
	scoped := Query{Scope: "project-alpha"}
	if scoped.IsEmpty() {
		t.Fatal("a scoped query reported empty")
	}
}

func TestQueryModeAndSortVocabulariesValidateThemselves(t *testing.T) {
	if len(AllQueryModes()) != 7 {
		t.Fatalf("mode catalog size = %d, want 7", len(AllQueryModes()))
	}
	for _, mode := range AllQueryModes() {
		if err := mode.Validate(); err != nil {
			t.Fatalf("catalog mode %q rejected itself: %v", mode, err)
		}
	}
	if err := QueryModeUnset.Validate(); err != nil {
		t.Fatalf("the unset mode axis was rejected: %v", err)
	}
	for _, field := range AllQuerySortFields() {
		if err := (QuerySort{Field: field}).Validate(); err != nil {
			t.Fatalf("sort field %q rejected itself: %v", field, err)
		}
	}
	if err := (QuerySort{}).Validate(); err != nil {
		t.Fatalf("the default sort was rejected: %v", err)
	}
}

func TestQueryClauseEqualIgnoresTheDerivedReloadFlag(t *testing.T) {
	clause := positiveClause(t, QueryFieldProjectTag, QueryOperatorIs, "infra")
	stripped := clause
	stripped.RequiresReload = false
	if !clause.Equal(stripped) {
		t.Fatal("clauses naming the same narrowing compared unequal over a derived field")
	}
	if clause.Equal(negativeClause(t, QueryFieldProjectTag, QueryOperatorIs, "infra")) {
		t.Fatal("clauses of opposite polarity compared equal")
	}
}
