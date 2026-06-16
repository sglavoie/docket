package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseSearchQuery_HyphenatedDottedTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTags []string
		wantText string
	}{
		{
			name:     "simple tag",
			input:    "#backend",
			wantTags: []string{"backend"},
			wantText: "",
		},
		{
			name:     "hyphenated tag",
			input:    "#my-tag",
			wantTags: []string{"my-tag"},
			wantText: "",
		},
		{
			name:     "dotted tag",
			input:    "#v2.0",
			wantTags: []string{"v2.0"},
			wantText: "",
		},
		{
			name:     "hyphen and dot tag",
			input:    "#my-app.v2",
			wantTags: []string{"my-app.v2"},
			wantText: "",
		},
		{
			name:     "tag with text",
			input:    "search text #my-tag",
			wantTags: []string{"my-tag"},
			wantText: "search text",
		},
		{
			name:     "multiple hyphenated tags",
			input:    "#front-end #back-end",
			wantTags: []string{"front-end", "back-end"},
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSearchQuery(tt.input)

			if len(result.Tags) != len(tt.wantTags) {
				t.Fatalf("ParseSearchQuery(%q).Tags = %v, want %v", tt.input, result.Tags, tt.wantTags)
			}
			for i, tag := range result.Tags {
				if tag != tt.wantTags[i] {
					t.Errorf("ParseSearchQuery(%q).Tags[%d] = %q, want %q", tt.input, i, tag, tt.wantTags[i])
				}
			}

			if result.TextQuery != tt.wantText {
				t.Errorf("ParseSearchQuery(%q).TextQuery = %q, want %q", tt.input, result.TextQuery, tt.wantText)
			}
		})
	}
}

func TestParseSearchQuery_PriorityAbbreviations(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantPriorities []int
		wantText       string
	}{
		{"numeric !1", "!1", []int{1}, ""},
		{"numeric !5", "!5", []int{5}, ""},
		{"range !1-3", "!1-3", []int{1, 2, 3}, ""},
		{"abbrev !h (high)", "!h", []int{1}, ""},
		{"abbrev !hi (high)", "!hi", []int{1}, ""},
		{"abbrev !c (critical)", "!c", []int{1}, ""},
		{"abbrev !cr (critical)", "!cr", []int{1}, ""},
		{"abbrev !u (urgent)", "!u", []int{1}, ""},
		{"abbrev !ur (urgent)", "!ur", []int{1}, ""},
		{"abbrev !m (medium)", "!m", []int{3}, ""},
		{"abbrev !me (medium)", "!me", []int{3}, ""},
		{"abbrev !med (medium)", "!med", []int{3}, ""},
		{"abbrev !n (normal)", "!n", []int{3}, ""},
		{"abbrev !no (normal)", "!no", []int{3}, ""},
		{"abbrev !l (low)", "!l", []int{5}, ""},
		{"abbrev !lo (low)", "!lo", []int{5}, ""},
		{"full word !high", "!high", []int{1}, ""},
		{"full word !critical", "!critical", []int{1}, ""},
		{"full word !urgent", "!urgent", []int{1}, ""},
		{"full word !normal", "!normal", []int{3}, ""},
		{"full word !medium", "!medium", []int{3}, ""},
		{"full word !low", "!low", []int{5}, ""},
		{"full word !lowest", "!lowest", []int{5}, ""},
		{"case insensitive !High", "!High", []int{1}, ""},
		{"case insensitive !MED", "!MED", []int{3}, ""},
		{"abbrev with text", "fix bug !h", []int{1}, "fix bug"},
		{"multiple abbrevs", "!h !lo", []int{1, 5}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSearchQuery(tt.input)

			if len(result.Priorities) != len(tt.wantPriorities) {
				t.Fatalf("ParseSearchQuery(%q).Priorities = %v, want %v", tt.input, result.Priorities, tt.wantPriorities)
			}
			for i, p := range result.Priorities {
				if p != tt.wantPriorities[i] {
					t.Errorf("ParseSearchQuery(%q).Priorities[%d] = %d, want %d", tt.input, i, p, tt.wantPriorities[i])
				}
			}

			if result.TextQuery != tt.wantText {
				t.Errorf("ParseSearchQuery(%q).TextQuery = %q, want %q", tt.input, result.TextQuery, tt.wantText)
			}
		})
	}
}

func TestParseSearchQuery_AllStatuses(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStatus []models.TaskStatus
		wantText   string
	}{
		{
			name:       "todo status",
			input:      "$todo",
			wantStatus: []models.TaskStatus{models.StatusTodo},
		},
		{
			name:       "in_progress status",
			input:      "$in_progress",
			wantStatus: []models.TaskStatus{models.StatusInProgress},
		},
		{
			name:       "review status",
			input:      "$review",
			wantStatus: []models.TaskStatus{models.StatusReview},
		},
		{
			name:       "standby status",
			input:      "$standby",
			wantStatus: []models.TaskStatus{models.StatusStandby},
		},
		{
			name:       "done status",
			input:      "$done",
			wantStatus: []models.TaskStatus{models.StatusDone},
		},
		{
			name:       "blocked status",
			input:      "$blocked",
			wantStatus: []models.TaskStatus{models.StatusBlocked},
		},
		{
			name:       "canceled status",
			input:      "$canceled",
			wantStatus: []models.TaskStatus{models.StatusCanceled},
		},
		{
			name:       "status with text",
			input:      "fix bug $review",
			wantStatus: []models.TaskStatus{models.StatusReview},
			wantText:   "fix bug",
		},
		{
			name:       "multiple statuses",
			input:      "$review $standby",
			wantStatus: []models.TaskStatus{models.StatusReview, models.StatusStandby},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSearchQuery(tt.input)

			if len(result.Statuses) != len(tt.wantStatus) {
				t.Fatalf("ParseSearchQuery(%q).Statuses = %v, want %v", tt.input, result.Statuses, tt.wantStatus)
			}
			for i, s := range result.Statuses {
				if s != tt.wantStatus[i] {
					t.Errorf("ParseSearchQuery(%q).Statuses[%d] = %q, want %q", tt.input, i, s, tt.wantStatus[i])
				}
			}

			if result.TextQuery != tt.wantText {
				t.Errorf("ParseSearchQuery(%q).TextQuery = %q, want %q", tt.input, result.TextQuery, tt.wantText)
			}
		})
	}
}

func TestTaskMatchesFilters_SectionMatchesGTDOnly(t *testing.T) {
	gtdTask := &models.Task{
		Title:   "GTD task",
		Section: "Next",
	}
	activeTask := &models.Task{
		Title: "Active task",
	}
	bothTask := &models.Task{
		Title:   "Both task",
		Section: "Next",
	}

	tests := []struct {
		name    string
		task    *models.Task
		section string
		want    bool
	}{
		{"GTD section matches GTD task", gtdTask, "Next", true},
		{"GTD section does not match empty task", activeTask, "Next", false},
		{"non-GTD section does not match empty task", activeTask, "Design", false},
		{"non-GTD section does not match GTD task", gtdTask, "Design", false},
		{"non-GTD section does not match task with GTD section", bothTask, "Design", false},
		{"GTD section matches task with both", bothTask, "Next", true},
		{"case insensitive non-GTD section does not match", activeTask, "design", false},
		{"no section filter matches all", gtdTask, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := &ParsedSearchQuery{Section: tt.section}
			got := TaskMatchesFilters(tt.task, filters)
			if got != tt.want {
				t.Errorf("TaskMatchesFilters(section=%q) = %v, want %v", tt.section, got, tt.want)
			}
		})
	}
}

func TestTaskMatchesFilters_SectionViaSearchQuery(t *testing.T) {
	task := &models.Task{
		Title:  "My task",
		Status: models.StatusTodo,
	}

	filters := ParseSearchQuery("/Design")
	if filters.Section != "Design" {
		t.Fatalf("ParseSearchQuery(/Design).Section = %q, want %q", filters.Section, "Design")
	}
	if TaskMatchesFilters(task, filters) {
		t.Error("TaskMatchesFilters should not match non-GTD section via /Design search query")
	}
}

func TestParseSearchQuery_HasIsNoPFilters(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantHas     []string
		wantNo      []string
		wantIs      []string
		wantProject string
		wantText    string
	}{
		{
			name:    "has:comment",
			input:   "has:comment",
			wantHas: []string{"comment"},
		},
		{
			name:   "is:pinned",
			input:  "is:pinned",
			wantIs: []string{"pinned"},
		},
		{
			name:   "no:description",
			input:  "no:description",
			wantNo: []string{"description"},
		},
		{
			name:        "p:myproj",
			input:       "p:myproj",
			wantProject: "myproj",
		},
		{
			name:    "has:desc alias",
			input:   "has:desc",
			wantHas: []string{"description"},
		},
		{
			name:    "has:deps alias",
			input:   "has:deps",
			wantHas: []string{"dependency"},
		},
		{
			name:    "has:subtasks alias",
			input:   "has:subtasks",
			wantHas: []string{"children"},
		},
		{
			name:   "no:deps alias",
			input:  "no:deps",
			wantNo: []string{"dependency"},
		},
		{
			name:     "combination with text",
			input:    "#backend has:comment is:pinned",
			wantHas:  []string{"comment"},
			wantIs:   []string{"pinned"},
			wantText: "",
		},
		{
			name:        "project: long form",
			input:       "project:myproj",
			wantProject: "myproj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSearchQuery(tt.input)

			if !stringSliceEqual(result.HasPresence, tt.wantHas) {
				t.Errorf("HasPresence = %v, want %v", result.HasPresence, tt.wantHas)
			}
			if !stringSliceEqual(result.NoPresence, tt.wantNo) {
				t.Errorf("NoPresence = %v, want %v", result.NoPresence, tt.wantNo)
			}
			if !stringSliceEqual(result.IsState, tt.wantIs) {
				t.Errorf("IsState = %v, want %v", result.IsState, tt.wantIs)
			}
			if result.ProjectSlug != tt.wantProject {
				t.Errorf("ProjectSlug = %q, want %q", result.ProjectSlug, tt.wantProject)
			}
			if tt.wantText != "" && result.TextQuery != tt.wantText {
				t.Errorf("TextQuery = %q, want %q", result.TextQuery, tt.wantText)
			}
		})
	}
}

func TestTaskMatchesFilters_HasPresence(t *testing.T) {
	future := time.Now().Add(365 * 24 * time.Hour)
	dueDate := &future
	tomorrow := time.Now().AddDate(0, 0, 1)

	withComment := &models.Task{Comments: []*models.Comment{{Text: "hi"}}}
	noComment := &models.Task{}
	withDue := &models.Task{DueDate: dueDate} //nolint:govet
	withAttachment := &models.Task{Attachments: []models.Attachment{{Filename: "f.txt"}}}
	withRecurrence := &models.Task{Recurrence: &models.RecurrencePattern{}}
	withChecklist := &models.Task{Checklist: []models.ChecklistItem{{Text: "a"}}}
	snoozed := &models.Task{SnoozedUntil: &tomorrow}

	tests := []struct {
		name    string
		task    *models.Task
		filters *ParsedSearchQuery
		want    bool
	}{
		{"has:comment match", withComment, &ParsedSearchQuery{HasPresence: []string{"comment"}}, true},
		{"has:comment no match", noComment, &ParsedSearchQuery{HasPresence: []string{"comment"}}, false},
		{"no:comment match", noComment, &ParsedSearchQuery{NoPresence: []string{"comment"}}, true},
		{"no:comment no match", withComment, &ParsedSearchQuery{NoPresence: []string{"comment"}}, false},
		{"has:due match", withDue, &ParsedSearchQuery{HasPresence: []string{"due"}}, true},
		{"no:due match", noComment, &ParsedSearchQuery{NoPresence: []string{"due"}}, true},
		{"has:attachment match", withAttachment, &ParsedSearchQuery{HasPresence: []string{"attachment"}}, true},
		{"has:recurrence match", withRecurrence, &ParsedSearchQuery{HasPresence: []string{"recurrence"}}, true},
		{"has:checklist match", withChecklist, &ParsedSearchQuery{HasPresence: []string{"checklist"}}, true},
		{"is:snoozed match", snoozed, &ParsedSearchQuery{IsState: []string{"snoozed"}}, true},
		{"is:snoozed no match", noComment, &ParsedSearchQuery{IsState: []string{"snoozed"}}, false},
		{"is:pinned no match", noComment, &ParsedSearchQuery{IsState: []string{"pinned"}}, false},
		{"is:pinned match", &models.Task{Pinned: true}, &ParsedSearchQuery{IsState: []string{"pinned"}}, true},
		{"is:leaf match (no children)", noComment, &ParsedSearchQuery{IsState: []string{"leaf"}}, true},
		{"is:root match (no parentID)", noComment, &ParsedSearchQuery{IsState: []string{"root"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskMatchesFilters(tt.task, tt.filters)
			if got != tt.want {
				t.Errorf("TaskMatchesFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskMatchesFilters_Combinations(t *testing.T) {
	future2 := time.Now().Add(365 * 24 * time.Hour)
	task := &models.Task{
		Tags:     []string{"backend"},
		Comments: []*models.Comment{{Text: "notes"}},
		Pinned:   true,
		DueDate:  &future2,
	}

	filters := ParseSearchQuery("#backend has:comment is:pinned")
	if !TaskMatchesFilters(task, filters) {
		t.Error("expected task to match #backend has:comment is:pinned")
	}

	filters2 := ParseSearchQuery("is:pinned no:due")
	if TaskMatchesFilters(task, filters2) {
		t.Error("expected task NOT to match is:pinned no:due (task has due date)")
	}
}

func TestHasTaskOnlyFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters *ParsedSearchQuery
		want    bool
	}{
		{"empty", &ParsedSearchQuery{}, false},
		{"text only", &ParsedSearchQuery{TextQuery: "foo"}, false},
		{"project slug only", &ParsedSearchQuery{ProjectSlug: "proj"}, false},
		{"has presence only", &ParsedSearchQuery{HasPresence: []string{"tag"}}, false},
		{"no presence only", &ParsedSearchQuery{NoPresence: []string{"description"}}, false},
		{"is state only", &ParsedSearchQuery{IsState: []string{"pinned"}}, false},
		{"tags only", &ParsedSearchQuery{Tags: []string{"backend"}}, false},
		{"priorities", &ParsedSearchQuery{Priorities: []int{1}}, true},
		{"statuses", &ParsedSearchQuery{Statuses: []models.TaskStatus{models.StatusTodo}}, true},
		{"types", &ParsedSearchQuery{Types: []models.TaskType{models.TaskTypeBug}}, true},
		{"due date", &ParsedSearchQuery{DueDateFilter: "today"}, true},
		{"section", &ParsedSearchQuery{Section: "next"}, true},
		{"active only", &ParsedSearchQuery{ActiveOnly: true}, true},
		{"task-only + shared", &ParsedSearchQuery{Priorities: []int{1}, Tags: []string{"x"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filters.HasTaskOnlyFilters(); got != tt.want {
				t.Errorf("HasTaskOnlyFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSearchQuery_CaseInsensitiveStatusAndDueDate(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStatus []models.TaskStatus
		wantDue    string
		wantText   string
	}{
		{"uppercase status", "$TODO", []models.TaskStatus{models.StatusTodo}, "", ""},
		{"mixed case status", "$Todo", []models.TaskStatus{models.StatusTodo}, "", ""},
		{"uppercase in_progress", "$IN_PROGRESS", []models.TaskStatus{models.StatusInProgress}, "", ""},
		{"mixed case Review", "$Review", []models.TaskStatus{models.StatusReview}, "", ""},
		{"uppercase due date", "@TODAY", nil, "today", ""},
		{"mixed case due date", "@Today", nil, "today", ""},
		{"uppercase overdue", "@OVERDUE", nil, "overdue", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ParseSearchQuery(tt.input)
			if len(r.Statuses) != len(tt.wantStatus) {
				t.Fatalf("Statuses = %v, want %v", r.Statuses, tt.wantStatus)
			}
			for i, s := range r.Statuses {
				if s != tt.wantStatus[i] {
					t.Errorf("Statuses[%d] = %q, want %q", i, s, tt.wantStatus[i])
				}
			}
			if r.DueDateFilter != tt.wantDue {
				t.Errorf("DueDateFilter = %q, want %q", r.DueDateFilter, tt.wantDue)
			}
			if r.TextQuery != tt.wantText {
				t.Errorf("TextQuery = %q, want %q", r.TextQuery, tt.wantText)
			}
		})
	}
}

// stringSliceEqual compares two string slices, treating nil and empty as equal.
func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
