package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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
			query:         "section:backlog task",
			wantRemaining: "task",
			wantOp:        "eq",
			wantValue:     "backlog",
		},
		{
			query:         "section:sprint-1|sprint-2",
			wantRemaining: "",
			wantOp:        "in",
			wantAlt:       []string{"sprint-1", "sprint-2"},
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

	// Find the quoted phrase group
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

	// Should have: bug|fix (OR group), "high priority" (exact), task
	if len(pos) != 3 {
		t.Errorf("expected 3 positive groups, got %d", len(pos))
	}
	// Should have: done|canceled (negative OR group)
	if len(neg) != 1 {
		t.Errorf("expected 1 negative group, got %d", len(neg))
	}
}

func TestTaskMatchesFieldFilters_Priority(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{Priority: 2}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "exact match",
			filters: []fieldFilter{{field: "priority", op: "eq", value: "2"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "priority", op: "eq", value: "1"}},
			want:    false,
		},
		{
			name:    "range match",
			filters: []fieldFilter{{field: "priority", op: "range", value: "1-3"}},
			want:    true,
		},
		{
			name:    "range no match",
			filters: []fieldFilter{{field: "priority", op: "range", value: "3-5"}},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_Status(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{Status: models.StatusInProgress}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "partial match",
			filters: []fieldFilter{{field: "status", op: "eq", value: "progress"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "status", op: "eq", value: "done"}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []fieldFilter{{field: "status", op: "in", alt: []string{"todo", "progress"}}},
			want:    true,
		},
		{
			name:    "in no match",
			filters: []fieldFilter{{field: "status", op: "in", alt: []string{"todo", "done"}}},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_Tag(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{Tags: []string{"bug", "urgent"}}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "exact match",
			filters: []fieldFilter{{field: "tag", op: "eq", value: "bug"}},
			want:    true,
		},
		{
			name:    "case insensitive",
			filters: []fieldFilter{{field: "tag", op: "eq", value: "BUG"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "tag", op: "eq", value: "feature"}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []fieldFilter{{field: "tag", op: "in", alt: []string{"feature", "urgent"}}},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_Section(t *testing.T) {
	m := &CommandPaletteModel{}
	task := &models.Task{Section: "backlog"}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "exact match",
			filters: []fieldFilter{{field: "section", op: "eq", value: "backlog"}},
			want:    true,
		},
		{
			name:    "case insensitive",
			filters: []fieldFilter{{field: "section", op: "eq", value: "BACKLOG"}},
			want:    true,
		},
		{
			name:    "no match",
			filters: []fieldFilter{{field: "section", op: "eq", value: "sprint"}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []fieldFilter{{field: "section", op: "in", alt: []string{"sprint", "backlog"}}},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_SectionCustom(t *testing.T) {
	m := &CommandPaletteModel{}
	// Task with CustomSection set (as happens after parser migration for non-GTD sections)
	task := &models.Task{Section: models.SectionInbox, CustomSection: "Sprint1"}
	item := PaletteItem{Category: ItemCategoryTasks, Task: task}

	tests := []struct {
		name    string
		filters []fieldFilter
		want    bool
	}{
		{
			name:    "match by CustomSection eq",
			filters: []fieldFilter{{field: "section", op: "eq", value: "Sprint1"}},
			want:    true,
		},
		{
			name:    "match by GTD Section eq",
			filters: []fieldFilter{{field: "section", op: "eq", value: models.SectionInbox}},
			want:    true,
		},
		{
			name:    "no match wrong section",
			filters: []fieldFilter{{field: "section", op: "eq", value: "Sprint2"}},
			want:    false,
		},
		{
			name:    "in match via CustomSection",
			filters: []fieldFilter{{field: "section", op: "in", alt: []string{"Sprint1", "Sprint2"}}},
			want:    true,
		},
		{
			name:    "in no match",
			filters: []fieldFilter{{field: "section", op: "in", alt: []string{"Sprint2", "Sprint3"}}},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.taskMatchesFieldFilters(item, tc.filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildSectionListIncludesCustomSections(t *testing.T) {
	project, err := models.NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{models.SectionNext, models.SectionInbox, "Sprint1"}

	task1, _ := models.NewTask("1", 1, models.TaskTypeTask, "Task 1")
	task1.Section = models.SectionInbox
	task1.CustomSection = "Sprint1"

	task2, _ := models.NewTask("2", 2, models.TaskTypeTask, "Task 2")
	task2.CustomSection = "Sprint1"

	project.Tasks = []*models.Task{task1, task2}

	sections := buildSectionList(project)

	// Find Sprint1 in the list and verify its count
	found := false
	for _, s := range sections {
		if s.Name == "Sprint1" {
			found = true
			if s.TaskCount != 2 {
				t.Errorf("expected 2 tasks in Sprint1, got %d", s.TaskCount)
			}
		}
	}
	if !found {
		t.Error("Sprint1 section not found in buildSectionList output")
	}
}

func TestTaskMatchesFieldFilters_Date(t *testing.T) {
	m := &CommandPaletteModel{}
	now := time.Now()
	today := util.StartOfDay(now)

	tests := []struct {
		name    string
		dueDate *time.Time
		filter  string
		want    bool
	}{
		{
			name:    "today match",
			dueDate: &today,
			filter:  "today",
			want:    true,
		},
		{
			name:    "overdue",
			dueDate: func() *time.Time { d := today.AddDate(0, 0, -1); return &d }(),
			filter:  "overdue",
			want:    true,
		},
		{
			name:    "not overdue",
			dueDate: &today,
			filter:  "overdue",
			want:    false,
		},
		{
			name:    "tomorrow",
			dueDate: func() *time.Time { d := today.AddDate(0, 0, 1); return &d }(),
			filter:  "tomorrow",
			want:    true,
		},
		{
			name:    "this week",
			dueDate: func() *time.Time { d := today.AddDate(0, 0, 3); return &d }(),
			filter:  "this-week",
			want:    true,
		},
		{
			name:    "no due date",
			dueDate: nil,
			filter:  "today",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{DueDate: tc.dueDate}
			item := PaletteItem{Category: ItemCategoryTasks, Task: task}
			filters := []fieldFilter{{field: "date", op: "eq", value: tc.filter}}

			got := m.taskMatchesFieldFilters(item, filters)
			if got != tc.want {
				t.Errorf("taskMatchesFieldFilters() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTaskMatchesFieldFilters_NonTaskItem(t *testing.T) {
	m := &CommandPaletteModel{}
	item := PaletteItem{Category: ItemCategoryCommands, Command: &Command{Name: "test"}}

	// Non-task items with no filters should pass
	got := m.taskMatchesFieldFilters(item, nil)
	if !got {
		t.Error("expected non-task item with no filters to match")
	}

	// Non-task items with filters should fail
	got = m.taskMatchesFieldFilters(item, []fieldFilter{{field: "priority", op: "eq", value: "1"}})
	if got {
		t.Error("expected non-task item with filters to not match")
	}
}

func TestTaskIDPattern(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"#123", true},
		{"#1", true},
		{"#0", true},
		{"#abc", false},
		{"123", false},
		{"#", false},
		{"# 123", false},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			got := taskIDPattern.MatchString(tc.query)
			if got != tc.want {
				t.Errorf("taskIDPattern.MatchString(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func TestInlineTaskPattern(t *testing.T) {
	tests := []struct {
		query     string
		wantMatch bool
		wantTitle string
	}{
		{"+New task", true, "New task"},
		{"+Task with spaces", true, "Task with spaces"},
		{"+", false, ""},
		{"+ ", true, " "},
		{"No plus", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			matches := inlineTaskPattern.FindStringSubmatch(tc.query)
			gotMatch := matches != nil
			if gotMatch != tc.wantMatch {
				t.Errorf("inlineTaskPattern.MatchString(%q) = %v, want %v", tc.query, gotMatch, tc.wantMatch)
			}
			if gotMatch && len(matches) > 1 && matches[1] != tc.wantTitle {
				t.Errorf("captured title = %q, want %q", matches[1], tc.wantTitle)
			}
		})
	}
}
