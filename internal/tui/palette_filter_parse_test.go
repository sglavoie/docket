package tui

import "testing"

func TestParseFieldFilters_Priority(t *testing.T) {
	tests := []struct {
		query            string
		wantRemaining    string
		wantFiltersCount int
		wantField        string
		wantOp           string
		wantValue        string
	}{
		{
			query:            "task priority:1",
			wantRemaining:    "task",
			wantFiltersCount: 1,
			wantField:        "priority",
			wantOp:           "eq",
			wantValue:        "1",
		},
		{
			query:            "priority:2-4 bug",
			wantRemaining:    "bug",
			wantFiltersCount: 1,
			wantField:        "priority",
			wantOp:           "range",
			wantValue:        "2-4",
		},
		{
			query:            "priority:3",
			wantRemaining:    "",
			wantFiltersCount: 1,
			wantField:        "priority",
			wantOp:           "eq",
			wantValue:        "3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			remaining, filters := parseFieldFilters(tc.query)

			if remaining != tc.wantRemaining {
				t.Errorf("remaining = %q, want %q", remaining, tc.wantRemaining)
			}
			if len(filters) != tc.wantFiltersCount {
				t.Errorf("got %d filters, want %d", len(filters), tc.wantFiltersCount)
				return
			}
			if tc.wantFiltersCount > 0 {
				f := filters[0]
				if f.field != tc.wantField {
					t.Errorf("field = %q, want %q", f.field, tc.wantField)
				}
				if f.op != tc.wantOp {
					t.Errorf("op = %q, want %q", f.op, tc.wantOp)
				}
				if f.value != tc.wantValue {
					t.Errorf("value = %q, want %q", f.value, tc.wantValue)
				}
			}
		})
	}
}

func TestParseFieldFilters_Status(t *testing.T) {
	tests := []struct {
		query         string
		wantRemaining string
		wantOp        string
		wantValue     string
		wantAlt       []string
	}{
		{
			query:         "status:todo task",
			wantRemaining: "task",
			wantOp:        "eq",
			wantValue:     "todo",
		},
		{
			query:         "status:todo|progress",
			wantRemaining: "",
			wantOp:        "in",
			wantAlt:       []string{"todo", "progress"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			remaining, filters := parseFieldFilters(tc.query)

			if remaining != tc.wantRemaining {
				t.Errorf("remaining = %q, want %q", remaining, tc.wantRemaining)
			}
			if len(filters) != 1 {
				t.Errorf("got %d filters, want 1", len(filters))
				return
			}
			f := filters[0]
			if f.field != "status" {
				t.Errorf("field = %q, want 'status'", f.field)
			}
			if f.op != tc.wantOp {
				t.Errorf("op = %q, want %q", f.op, tc.wantOp)
			}
			if tc.wantAlt != nil {
				if len(f.alt) != len(tc.wantAlt) {
					t.Errorf("alt = %v, want %v", f.alt, tc.wantAlt)
				}
			} else if f.value != tc.wantValue {
				t.Errorf("value = %q, want %q", f.value, tc.wantValue)
			}
		})
	}
}

func TestParseFieldFilters_DatePredicates(t *testing.T) {
	tests := []struct {
		query         string
		wantRemaining string
		wantValue     string
	}{
		{"@overdue task", "task", "overdue"},
		{"@today", "", "today"},
		{"@tomorrow", "", "tomorrow"},
		{"@this-week task", "task", "this-week"},
		{"@thisweek", "", "thisweek"},
		{"@none", "", "none"},
		{"@none task", "task", "none"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			remaining, filters := parseFieldFilters(tc.query)

			if remaining != tc.wantRemaining {
				t.Errorf("remaining = %q, want %q", remaining, tc.wantRemaining)
			}
			if len(filters) != 1 {
				t.Errorf("got %d filters, want 1", len(filters))
				return
			}
			f := filters[0]
			if f.field != "date" {
				t.Errorf("field = %q, want 'date'", f.field)
			}
			if f.op != "eq" {
				t.Errorf("op = %q, want 'eq'", f.op)
			}
			if f.value != tc.wantValue {
				t.Errorf("value = %q, want %q", f.value, tc.wantValue)
			}
		})
	}
}

func TestParseFieldFilters_Tag(t *testing.T) {
	tests := []struct {
		query         string
		wantRemaining string
		wantOp        string
		wantValue     string
		wantAlt       []string
	}{
		{
			query:         "tag:bug fix",
			wantRemaining: "fix",
			wantOp:        "eq",
			wantValue:     "bug",
		},
		{
			query:         "tag:bug|feature",
			wantRemaining: "",
			wantOp:        "in",
			wantAlt:       []string{"bug", "feature"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			remaining, filters := parseFieldFilters(tc.query)

			if remaining != tc.wantRemaining {
				t.Errorf("remaining = %q, want %q", remaining, tc.wantRemaining)
			}
			if len(filters) != 1 {
				t.Errorf("got %d filters, want 1", len(filters))
				return
			}
			f := filters[0]
			if f.field != "tag" {
				t.Errorf("field = %q, want 'tag'", f.field)
			}
			if f.op != tc.wantOp {
				t.Errorf("op = %q, want %q", f.op, tc.wantOp)
			}
		})
	}
}

func TestParseFieldFilters_Section(t *testing.T) {
	tests := []struct {
		query         string
		wantRemaining string
		wantOp        string
		wantValue     string
		wantAlt       []string
	}{
		{
			query:         "section:inbox task",
			wantRemaining: "task",
			wantOp:        "eq",
			wantValue:     "inbox",
		},
		{
			query:         "section:next|waiting",
			wantRemaining: "",
			wantOp:        "in",
			wantAlt:       []string{"next", "waiting"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			remaining, filters := parseFieldFilters(tc.query)

			if remaining != tc.wantRemaining {
				t.Errorf("remaining = %q, want %q", remaining, tc.wantRemaining)
			}
			if len(filters) != 1 {
				t.Errorf("got %d filters, want 1", len(filters))
				return
			}
			f := filters[0]
			if f.field != "section" {
				t.Errorf("field = %q, want 'section'", f.field)
			}
			if f.op != tc.wantOp {
				t.Errorf("op = %q, want %q", f.op, tc.wantOp)
			}
		})
	}
}

func TestParseFieldFilters_MultipleFilters(t *testing.T) {
	query := "priority:1 status:todo @today task"
	remaining, filters := parseFieldFilters(query)

	if remaining != "task" {
		t.Errorf("remaining = %q, want 'task'", remaining)
	}
	if len(filters) != 3 {
		t.Errorf("got %d filters, want 3", len(filters))
	}
}

func TestParseFieldFilters_NoFilters(t *testing.T) {
	query := "just a regular search"
	remaining, filters := parseFieldFilters(query)

	if remaining != query {
		t.Errorf("remaining = %q, want %q", remaining, query)
	}
	if len(filters) != 0 {
		t.Errorf("got %d filters, want 0", len(filters))
	}
}

func TestParseSearchTerms_Simple(t *testing.T) {
	pos, neg := parseSearchTerms("task bug")

	if len(pos) != 2 {
		t.Errorf("expected 2 positive groups, got %d", len(pos))
	}
	if len(neg) != 0 {
		t.Errorf("expected 0 negative groups, got %d", len(neg))
	}
}

func TestParseSearchTerms_Negation(t *testing.T) {
	pos, neg := parseSearchTerms("task -done")

	if len(pos) != 1 {
		t.Errorf("expected 1 positive group, got %d", len(pos))
	}
	if len(neg) != 1 {
		t.Errorf("expected 1 negative group, got %d", len(neg))
	}
	if neg[0].alternatives[0] != "done" {
		t.Errorf("expected negative term 'done', got %q", neg[0].alternatives[0])
	}
}

func TestParseSearchTerms_ORGroups(t *testing.T) {
	pos, neg := parseSearchTerms("bug|fix|issue")

	if len(pos) != 1 {
		t.Errorf("expected 1 positive group, got %d", len(pos))
	}
	if len(neg) != 0 {
		t.Errorf("expected 0 negative groups, got %d", len(neg))
	}
	if len(pos[0].alternatives) != 3 {
		t.Errorf("expected 3 alternatives, got %d", len(pos[0].alternatives))
	}
}

func TestParseSearchTerms_NegativeORGroup(t *testing.T) {
	pos, neg := parseSearchTerms("task -done|canceled")

	if len(pos) != 1 {
		t.Errorf("expected 1 positive group, got %d", len(pos))
	}
	if len(neg) != 1 {
		t.Errorf("expected 1 negative group, got %d", len(neg))
	}
	if len(neg[0].alternatives) != 2 {
		t.Errorf("expected 2 negative alternatives, got %d", len(neg[0].alternatives))
	}
}

func TestParseSearchTerms_QuotedPhrase(t *testing.T) {
	pos, neg := parseSearchTerms(`"fix bug" task`)

	if len(pos) != 2 {
		t.Errorf("expected 2 positive groups, got %d", len(pos))
	}
	if len(neg) != 0 {
		t.Errorf("expected 0 negative groups, got %d", len(neg))
	}

	foundQuoted := false
	for _, g := range pos {
		if g.isExact && len(g.alternatives) == 1 && g.alternatives[0] == "fix bug" {
			foundQuoted = true
			break
		}
	}
	if !foundQuoted {
		t.Error("expected to find exact phrase 'fix bug'")
	}
}

func TestParseSearchTerms_NegativeQuotedPhrase(t *testing.T) {
	pos, neg := parseSearchTerms(`task -"not this"`)

	if len(pos) != 1 {
		t.Errorf("expected 1 positive group, got %d", len(pos))
	}
	if len(neg) != 1 {
		t.Errorf("expected 1 negative group, got %d", len(neg))
	}
	if !neg[0].isExact {
		t.Error("expected negative group to be exact match")
	}
	if neg[0].alternatives[0] != "not this" {
		t.Errorf("expected negative phrase 'not this', got %q", neg[0].alternatives[0])
	}
}

func TestParseSearchTerms_SingleQuotes(t *testing.T) {
	pos, _ := parseSearchTerms(`'exact match'`)

	if len(pos) != 1 {
		t.Errorf("expected 1 positive group, got %d", len(pos))
	}
	if !pos[0].isExact {
		t.Error("expected group to be exact match")
	}
	if pos[0].alternatives[0] != "exact match" {
		t.Errorf("expected phrase 'exact match', got %q", pos[0].alternatives[0])
	}
}

func TestParseSearchTerms_EmptyQuery(t *testing.T) {
	pos, neg := parseSearchTerms("")

	if len(pos) != 0 {
		t.Errorf("expected 0 positive groups, got %d", len(pos))
	}
	if len(neg) != 0 {
		t.Errorf("expected 0 negative groups, got %d", len(neg))
	}
}

func TestParseSearchTerms_OnlyNegation(t *testing.T) {
	pos, neg := parseSearchTerms("-done -canceled")

	if len(pos) != 0 {
		t.Errorf("expected 0 positive groups, got %d", len(pos))
	}
	if len(neg) != 2 {
		t.Errorf("expected 2 negative groups, got %d", len(neg))
	}
}

func TestParseSearchTerms_MixedComplex(t *testing.T) {
	pos, neg := parseSearchTerms(`bug|fix "high priority" -done|canceled task`)

	if len(pos) != 3 {
		t.Errorf("expected 3 positive groups, got %d", len(pos))
	}
	if len(neg) != 1 {
		t.Errorf("expected 1 negative group, got %d", len(neg))
	}
}
