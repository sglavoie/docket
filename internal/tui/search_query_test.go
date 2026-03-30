package tui

import (
	"testing"

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

func TestTaskMatchesFilters_SectionMatchesCustomSection(t *testing.T) {
	gtdTask := &models.Task{
		Title:   "GTD task",
		Section: "Next",
	}
	customTask := &models.Task{
		Title:         "Custom task",
		CustomSection: "Design",
	}
	bothTask := &models.Task{
		Title:         "Both task",
		Section:       "Next",
		CustomSection: "Design",
	}

	tests := []struct {
		name    string
		task    *models.Task
		section string
		want    bool
	}{
		{"GTD section matches GTD task", gtdTask, "Next", true},
		{"GTD section does not match custom task", customTask, "Next", false},
		{"custom section matches custom task", customTask, "Design", true},
		{"custom section does not match GTD task", gtdTask, "Design", false},
		{"custom section matches task with both", bothTask, "Design", true},
		{"GTD section matches task with both", bothTask, "Next", true},
		{"case insensitive custom section", customTask, "design", true},
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
		Title:         "My task",
		CustomSection: "Design",
		Status:        models.StatusTodo,
	}

	filters := ParseSearchQuery("/Design")
	if filters.Section != "Design" {
		t.Fatalf("ParseSearchQuery(/Design).Section = %q, want %q", filters.Section, "Design")
	}
	if !TaskMatchesFilters(task, filters) {
		t.Error("TaskMatchesFilters should match custom section via /Design search query")
	}
}
