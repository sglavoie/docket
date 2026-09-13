package models

import "testing"

func TestQueryClauseStringRendersEveryFieldsToken(t *testing.T) {
	cases := []struct {
		token    string
		field    QueryField
		operator QueryOperator
		value    string
	}{
		{`"login form"`, QueryFieldText, QueryOperatorContains, "login form"},
		{"~project-beta:task-7", QueryFieldDependency, QueryOperatorIs, "project-beta:task-7"},
		{"!2", QueryFieldPriority, QueryOperatorIs, "2"},
		{"!2+", QueryFieldPriority, QueryOperatorAtLeast, "2"},
		{"!0", QueryFieldPriority, QueryOperatorNone, ""},
		{"#auth", QueryFieldTag, QueryOperatorIs, "auth"},
		{"#auth*", QueryFieldTag, QueryOperatorPrefix, "auth"},
		{"##home", QueryFieldContext, QueryOperatorIs, "home"},
		{"##ho*", QueryFieldContext, QueryOperatorPrefix, "ho"},
		{"##0", QueryFieldContext, QueryOperatorNone, ""},
		{"@overdue", QueryFieldDue, QueryOperatorIs, QueryDueOverdue},
		{"@2026-09-01", QueryFieldDue, QueryOperatorIs, "2026-09-01"},
		{"@+3d", QueryFieldDue, QueryOperatorIs, "+3d"},
		{"@0", QueryFieldDue, QueryOperatorNone, ""},
		{"@@2026-09-01", QueryFieldDefer, QueryOperatorIs, "2026-09-01"},
		{"$waiting", QueryFieldState, QueryOperatorIs, string(StateWaiting)},
		{"*pin", QueryFieldPinned, QueryOperatorPresent, ""},
		{"=review", QueryFieldReview, QueryOperatorPresent, ""},
		{"?review", QueryFieldNeedsReview, QueryOperatorPresent, ""},
		{"?stale", QueryFieldStale, QueryOperatorPresent, ""},
		{"?deferred", QueryFieldDeferred, QueryOperatorPresent, ""},
		{"?completed", QueryFieldCompleted, QueryOperatorPresent, ""},
		{"?completed:90d", QueryFieldCompleted, QueryOperatorAtLeast, "90d"},
		{"?parent", QueryFieldParent, QueryOperatorPresent, ""},
		{"?child", QueryFieldChild, QueryOperatorPresent, ""},
		{"?leaf", QueryFieldLeaf, QueryOperatorPresent, ""},
		{"?blocked", QueryFieldBlocked, QueryOperatorPresent, ""},
		{"?blocking", QueryFieldBlocking, QueryOperatorPresent, ""},
		{"+#infra", QueryFieldProjectTag, QueryOperatorIs, "infra"},
		{"+#0", QueryFieldProjectTag, QueryOperatorNone, ""},
		{"+parked", QueryFieldParked, QueryOperatorIs, QueryPopulationInclude},
		{"+parked:only", QueryFieldParked, QueryOperatorIs, QueryPopulationOnly},
		{"+archived", QueryFieldArchived, QueryOperatorIs, QueryPopulationInclude},
		{"+archived:only", QueryFieldArchived, QueryOperatorIs, QueryPopulationOnly},
	}

	covered := make(map[QueryField]bool, len(cases))
	for _, testCase := range cases {
		t.Run(testCase.token, func(t *testing.T) {
			covered[testCase.field] = true
			positive := positiveClause(t, testCase.field, testCase.operator, testCase.value)
			if got := positive.String(); got != testCase.token {
				t.Fatalf("clause string = %q, want %q", got, testCase.token)
			}
			negative := negativeClause(t, testCase.field, testCase.operator, testCase.value)
			if got, want := negative.String(), "-"+testCase.token; got != want {
				t.Fatalf("negated clause string = %q, want %q", got, want)
			}
		})
	}
	for _, field := range QueryFieldOrder() {
		if !covered[field] {
			t.Fatalf("field %q renders no token in this table", field)
		}
	}
}

func TestQueryStringRoundTripsThroughParseQuery(t *testing.T) {
	inputs := []string{
		"",
		`"login form" !1+ #auth #ui -#wip ##home @overdue $doing *pin =review ?review -?review ?stale`,
		"~project-beta:task-7 !0 ##0 @0 +#0",
		"@+6h @@2026-09-01 ?completed:90d ?parent -?child ?leaf ?blocked -?blocking",
		"+parked +archived:only +#infra",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			parsed, err := ParseQuery(input)
			if err != nil {
				t.Fatalf("ParseQuery(%q): %v", input, err)
			}
			rendered := parsed.String()
			reparsed, err := ParseQuery(rendered)
			if err != nil {
				t.Fatalf("ParseQuery(%q): %v", rendered, err)
			}
			if !parsed.Equal(reparsed) {
				t.Fatalf("round trip changed the query: %q then %q", rendered, reparsed.String())
			}
			if reparsed.String() != rendered {
				t.Fatalf("rendering is not idempotent: %q then %q", rendered, reparsed.String())
			}
		})
	}
}

func TestParseQueryNormalizesClauseOrderAndReloadObligations(t *testing.T) {
	parsed, err := ParseQuery("?stale #ui #auth !2+ +parked")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if got, want := parsed.String(), "!2+ #auth #ui ?stale +parked"; got != want {
		t.Fatalf("parsed string = %q, want %q", got, want)
	}
	if !parsed.RequiresReload() {
		t.Fatal("a parsed population clause lost its reload obligation")
	}
	if parsed.Mode != QueryModeUnset || parsed.Scope != "" || parsed.Sort != (QuerySort{}) {
		t.Fatal("ParseQuery invented an axis the string does not carry")
	}
}

func TestParseQueryRefusesWhatStringNeverEmits(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"unknown sigil", "&https://example.com"},
		{"unterminated quote", `"login`},
		{"unknown derived token", "?orphan"},
		{"derived token carrying a value", "?stale:90"},
		{"review attention carrying a value", "?review:1d"},
		{"unknown population token", "+unfiled"},
		{"unknown state", "$review"},
		{"priority out of range", "!7"},
		{"bare word", "login"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := ParseQuery(testCase.input); err == nil {
				t.Fatalf("ParseQuery(%q) succeeded, want a refusal", testCase.input)
			}
		})
	}
}

func TestParseQueryKeepsAQuotedTextValueWhole(t *testing.T) {
	parsed, err := ParseQuery(`"login form" #auth`)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if len(parsed.Clauses) != 2 {
		t.Fatalf("clause count = %d, want 2", len(parsed.Clauses))
	}
	if parsed.Clauses[0].Field != QueryFieldText || parsed.Clauses[0].Value != "login form" {
		t.Fatalf("text clause = %+v, want the whole quoted value", parsed.Clauses[0])
	}
	escaped, err := ParseQuery(`"say \"hi\" now"`)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if got, want := escaped.Clauses[0].Value, `say "hi" now`; got != want {
		t.Fatalf("escaped text value = %q, want %q", got, want)
	}
	if got, want := escaped.String(), `"say \"hi\" now"`; got != want {
		t.Fatalf("escaped text renders as %q, want %q", got, want)
	}
}
