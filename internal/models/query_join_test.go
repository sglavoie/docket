package models

import "testing"

// The four tests below are the join law itself, one test per rule:
//
//   - across fields, AND;
//   - within a single-valued field, OR;
//   - within a multi-valued field, AND;
//   - negative clauses, AND-of-nots, at any cardinality and on either join.
//
// They are named after the rules rather than the fields so a later change to the
// vocabulary cannot quietly drop one of them.

func TestQueryJoinLawIsANDAcrossFields(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldPriority, QueryOperatorIs, "2"),
		positiveClause(t, QueryFieldContext, QueryOperatorIs, "home"),
		positiveClause(t, QueryFieldState, QueryOperatorIs, string(StateDoing)),
	}}

	both := queryTestTask()
	both.Context = "home"
	if !query.Admits(both, project, now) {
		t.Fatal("a task satisfying every field was refused")
	}

	for _, mutate := range []struct {
		name string
		fn   func(*Task)
	}{
		{"priority differs", func(task *Task) { task.Priority = PriorityHigh }},
		{"context differs", func(task *Task) { task.Context = "work" }},
		{"state differs", func(task *Task) { task.WorkflowState = StateBacklog }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			task := queryTestTask()
			task.Context = "home"
			mutate.fn(task)
			if query.Admits(task, project, now) {
				t.Fatalf("a task failing one field was admitted: %s", mutate.name)
			}
		})
	}
}

func TestQueryJoinLawIsORWithinASingleValuedField(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldContext, QueryOperatorIs, "home"),
		positiveClause(t, QueryFieldContext, QueryOperatorIs, "work"),
	}}

	for _, context := range []string{"home", "work"} {
		task := queryTestTask()
		task.Context = context
		if !query.Admits(task, project, now) {
			t.Fatalf("context %q was refused by a union of contexts naming it", context)
		}
	}

	other := queryTestTask()
	other.Context = "errand"
	if query.Admits(other, project, now) {
		t.Fatal("a context the union does not name was admitted")
	}

	// A single-valued field can hold only one value at a time, so ANDing the two
	// clauses would make this query unsatisfiable rather than merely narrow.
	unset := queryTestTask()
	if query.Admits(unset, project, now) {
		t.Fatal("a task with no context was admitted by a union of named contexts")
	}
}

func TestQueryJoinLawIsANDWithinAMultiValuedField(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
		positiveClause(t, QueryFieldTag, QueryOperatorIs, "ui"),
	}}

	both := queryTestTask()
	both.Tags = []string{"auth", "ui", "spike"}
	if !query.Admits(both, project, now) {
		t.Fatal("a task carrying both tags was refused")
	}

	for _, tags := range [][]string{{"auth"}, {"ui"}, nil} {
		task := queryTestTask()
		task.Tags = tags
		if query.Admits(task, project, now) {
			t.Fatalf("a task carrying %v was admitted by an intersection of two tags", tags)
		}
	}
}

func TestQueryJoinLawIsANDOfNotsForNegativeClausesAtAnyCardinality(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()

	cases := []struct {
		name    string
		clauses []QueryClause
		tags    []string
		context string
		admits  bool
	}{
		{
			name:    "one negative on a multi-valued field refuses what it names",
			clauses: []QueryClause{negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth")},
			tags:    []string{"auth"},
			admits:  false,
		},
		{
			name:    "one negative on a multi-valued field admits everything else",
			clauses: []QueryClause{negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth")},
			tags:    []string{"ui"},
			admits:  true,
		},
		{
			name: "two negatives on a multi-valued field refuse either name",
			clauses: []QueryClause{
				negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
				negativeClause(t, QueryFieldTag, QueryOperatorIs, "ui"),
			},
			tags:   []string{"ui"},
			admits: false,
		},
		{
			name: "two negatives on a multi-valued field admit neither name",
			clauses: []QueryClause{
				negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
				negativeClause(t, QueryFieldTag, QueryOperatorIs, "ui"),
			},
			tags:   []string{"spike"},
			admits: true,
		},
		{
			name: "two negatives on a single-valued field still AND rather than union",
			clauses: []QueryClause{
				negativeClause(t, QueryFieldContext, QueryOperatorIs, "home"),
				negativeClause(t, QueryFieldContext, QueryOperatorIs, "work"),
			},
			context: "work",
			admits:  false,
		},
		{
			name: "two negatives on a single-valued field admit an unnamed value",
			clauses: []QueryClause{
				negativeClause(t, QueryFieldContext, QueryOperatorIs, "home"),
				negativeClause(t, QueryFieldContext, QueryOperatorIs, "work"),
			},
			context: "errand",
			admits:  true,
		},
		{
			name: "a negative overrides a positive naming the same value",
			clauses: []QueryClause{
				positiveClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
				negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth"),
			},
			tags:   []string{"auth"},
			admits: false,
		},
		{
			name: "a negative narrows a positive union on the same field",
			clauses: []QueryClause{
				positiveClause(t, QueryFieldContext, QueryOperatorPrefix, "ho"),
				negativeClause(t, QueryFieldContext, QueryOperatorIs, "home"),
			},
			context: "home",
			admits:  false,
		},
		{
			name:    "a field carrying only negatives admits a task the field cannot match",
			clauses: []QueryClause{negativeClause(t, QueryFieldTag, QueryOperatorIs, "auth")},
			admits:  true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			task.Tags = testCase.tags
			task.Context = testCase.context
			query := Query{Clauses: testCase.clauses}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}
