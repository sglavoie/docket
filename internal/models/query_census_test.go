package models

import (
	"testing"
	"time"
)

func censusTestQuery(t *testing.T, clauses ...QueryClause) Query {
	t.Helper()
	query, err := NewQuery(QueryModeUnset, "", QuerySort{}, clauses...)
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	return query
}

func censusTestClause(t *testing.T, field QueryField, polarity QueryPolarity) QueryClause {
	t.Helper()
	clause, err := NewQueryClause(field, polarity, QueryOperatorPresent, "")
	if err != nil {
		t.Fatalf("NewQueryClause(%s): %v", field, err)
	}
	return clause
}

// TestQueryAdmitsInPopulationAnswersCensusFields holds the census fields to the
// population: parent and leaf read open direct children across the flat task
// list, blocking reads who names the task, and the join law still applies.
func TestQueryAdmitsInPopulationAnswersCensusFields(t *testing.T) {
	now := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	parentID := "parent"
	parent := &Task{ID: parentID, Title: "Parent"}
	child := &Task{ID: "child", Title: "Child", ParentID: &parentID}
	parent.Children = []*Task{child}
	blocker := &Task{ID: "blocker", Title: "Blocker"}
	project := &Project{ID: "alpha", Slug: "alpha", Tasks: []*Task{parent, child, blocker}}
	dependent := &Task{ID: "dependent", Title: "Dependent", Dependencies: []TaskReference{{ProjectID: "alpha", TaskID: "blocker"}}}
	other := &Project{ID: "beta", Slug: "beta", Tasks: []*Task{dependent}}
	census := NewQueryCensus([]*Project{project, other})

	type row struct {
		task    *Task
		project *Project
	}
	rows := []row{{parent, project}, {child, project}, {blocker, project}, {dependent, other}}
	cases := []struct {
		name  string
		query Query
		want  []string
	}{
		{"parent", censusTestQuery(t, censusTestClause(t, QueryFieldParent, QueryPolarityPositive)), []string{"Parent"}},
		{"leaf", censusTestQuery(t, censusTestClause(t, QueryFieldLeaf, QueryPolarityPositive)), []string{"Child", "Blocker", "Dependent"}},
		{"child", censusTestQuery(t, censusTestClause(t, QueryFieldChild, QueryPolarityPositive)), []string{"Child"}},
		{"not child", censusTestQuery(t, censusTestClause(t, QueryFieldChild, QueryPolarityNegative)), []string{"Parent", "Blocker", "Dependent"}},
		{"blocked", censusTestQuery(t, censusTestClause(t, QueryFieldBlocked, QueryPolarityPositive)), []string{"Dependent"}},
		{"blocking across projects", censusTestQuery(t, censusTestClause(t, QueryFieldBlocking, QueryPolarityPositive)), []string{"Blocker"}},
		{"census and row clauses AND", censusTestQuery(t,
			censusTestClause(t, QueryFieldLeaf, QueryPolarityPositive),
			censusTestClause(t, QueryFieldBlocked, QueryPolarityNegative),
			censusTestClause(t, QueryFieldChild, QueryPolarityNegative),
		), []string{"Blocker"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, r := range rows {
				if tc.query.AdmitsInPopulation(r.task, r.project, census, now) {
					got = append(got, r.task.Title)
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			}
		})
	}

	// Admits alone cannot know who names a task, which is why the census exists.
	blocking := censusTestQuery(t, censusTestClause(t, QueryFieldBlocking, QueryPolarityPositive))
	if blocking.Admits(blocker, project, now) {
		t.Fatal("Admits answered a census field it has no population for")
	}
	if blocking.AdmitsInPopulation(nil, project, census, now) {
		t.Fatal("a nil task was admitted")
	}
}

// TestCompareTasksBySortFieldOrdersEachField pins the shared sort comparator:
// ascending on each field, undated work after dated, and zero on the default.
func TestCompareTasksBySortFieldOrdersEachField(t *testing.T) {
	early := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	late := early.AddDate(0, 0, 3)
	a := &Task{Title: "alpha", Priority: 1, DueDate: &early, CreatedAt: early, UpdatedAt: early, WorkflowState: StateInbox}
	b := &Task{Title: "Beta", Priority: 0, DueDate: &late, CreatedAt: late, UpdatedAt: late, WorkflowState: StateDoing}
	undated := &Task{Title: "gamma"}
	for _, field := range []QuerySortField{QuerySortPriority, QuerySortDue, QuerySortCreated, QuerySortStatus, QuerySortUpdated, QuerySortTitle} {
		if c := CompareTasksBySortField(a, b, field); c >= 0 {
			t.Errorf("%s: compare(a, b) = %d, want a first", field, c)
		}
		if c := CompareTasksBySortField(b, a, field); c <= 0 {
			t.Errorf("%s: compare(b, a) = %d, want b second", field, c)
		}
	}
	if c := CompareTasksBySortField(undated, b, QuerySortDue); c <= 0 {
		t.Errorf("an undated task sorted ahead of a dated one: %d", c)
	}
	if c := CompareTasksBySortField(a, b, QuerySortDefault); c != 0 {
		t.Errorf("the default order compared %d, want a tie the caller breaks", c)
	}
}
