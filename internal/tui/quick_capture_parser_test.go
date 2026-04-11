package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickCapture(t *testing.T) {
	project := &models.Project{
		ID:       "test-project",
		Name:     "Test Project",
		Sections: []string{"Active", "Backlog", "Pending"},
		Tasks: []*models.Task{
			{ID: "epic-1", ShortID: 1, Type: models.TaskTypeEpic, Title: "User Authentication"},
			{ID: "story-2", ShortID: 2, Type: models.TaskTypeStory, Title: "Login Flow"},
			{ID: "task-3", ShortID: 3, Type: models.TaskTypeTask, Title: "Implement button"},
			{ID: "task-4", ShortID: 4, Type: models.TaskTypeTask, Title: "Implement search"},
		},
	}

	tests := []struct {
		name           string
		input          string
		wantTitle      string
		wantPriority   int
		wantTags       []string
		wantHasDueDate bool
		wantParentID   *string
		wantTaskType   models.TaskType
		wantSection    string
		wantHasRecur   bool
		wantStatus     models.TaskStatus
		wantActivate   bool
		wantErr        bool
		errContains    string
	}{
		{
			name:         "simple title only",
			input:        "Fix login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 3,
			wantTags:     nil,
		},
		{
			name:         "title with priority",
			input:        "Fix critical bug !1",
			wantTitle:    "Fix critical bug",
			wantPriority: 1,
			wantTags:     nil,
		},
		{
			name:         "title with low priority",
			input:        "Cleanup code !5",
			wantTitle:    "Cleanup code",
			wantPriority: 5,
			wantTags:     nil,
		},
		{
			name:         "priority at start",
			input:        "!2 Update documentation",
			wantTitle:    "Update documentation",
			wantPriority: 2,
			wantTags:     nil,
		},
		{
			name:         "single tag",
			input:        "Add feature #backend",
			wantTitle:    "Add feature",
			wantPriority: 3,
			wantTags:     []string{"backend"},
		},
		{
			name:         "multiple tags",
			input:        "Fix bug #backend #urgent #hotfix",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantTags:     []string{"backend", "urgent", "hotfix"},
		},
		{
			name:         "tags at different positions",
			input:        "#frontend Add button #ui to sidebar",
			wantTitle:    "Add button to sidebar",
			wantPriority: 3,
			wantTags:     []string{"frontend", "ui"},
		},
		{
			name:         "priority and tags combined",
			input:        "Critical fix !1 #hotfix #production",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"hotfix", "production"},
		},
		{
			name:           "with due date tomorrow",
			input:          "Review PR @tomorrow",
			wantTitle:      "Review PR",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date relative days",
			input:          "Submit report @+3d",
			wantTitle:      "Submit report",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date weekday",
			input:          "Team meeting @friday",
			wantTitle:      "Team meeting",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date exact",
			input:          "Project deadline @2025-12-31",
			wantTitle:      "Project deadline",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date in 2 weeks",
			input:          "Review code @in 2 weeks",
			wantTitle:      "Review code",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date in 3 days",
			input:          "Submit report @in 3 days",
			wantTitle:      "Submit report",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date next monday",
			input:          "Team sync @next monday",
			wantTitle:      "Team sync",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date next friday",
			input:          "Demo prep @next friday",
			wantTitle:      "Demo prep",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date next week",
			input:          "Planning @next week",
			wantTitle:      "Planning",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "multi-word date with other metadata",
			input:          "Critical fix !1 #urgent @in 2 weeks /Active",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"urgent"},
			wantHasDueDate: true,
			wantSection:    "Active",
		},
		{
			name:           "all metadata combined",
			input:          "Urgent task !1 #backend #critical @tomorrow",
			wantTitle:      "Urgent task",
			wantPriority:   1,
			wantTags:       []string{"backend", "critical"},
			wantHasDueDate: true,
		},
		{
			name:         "parent by short ID",
			input:        "Subtask under epic >1",
			wantTitle:    "Subtask under epic",
			wantPriority: 3,
			wantParentID: strPtr("epic-1"),
		},
		{
			name:         "parent by title prefix",
			input:        "Implementation detail >Login",
			wantTitle:    "Implementation detail",
			wantPriority: 3,
			wantParentID: strPtr("story-2"),
		},
		{
			name:           "full syntax with parent",
			input:          "Complete login form !2 #frontend @+1w >2",
			wantTitle:      "Complete login form",
			wantPriority:   2,
			wantTags:       []string{"frontend"},
			wantHasDueDate: true,
			wantParentID:   strPtr("story-2"),
		},
		{
			name:        "empty after extraction",
			input:       "!1 #tag @tomorrow",
			wantErr:     true,
			errContains: "title cannot be empty",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:        "invalid due date",
			input:       "Task with bad date @notadate",
			wantErr:     true,
			errContains: "invalid date",
		},
		{
			name:        "parent not found",
			input:       "Task with missing parent >999",
			wantErr:     true,
			errContains: "parent not found",
		},
		{
			name:         "extra whitespace handling",
			input:        "  Fix   the   bug   !2   #backend   ",
			wantTitle:    "Fix the bug",
			wantPriority: 2,
			wantTags:     []string{"backend"},
		},
		{
			name:         "section assignment",
			input:        "Fix bug /Active",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantSection:  "Active",
		},
		{
			name:         "section case insensitive",
			input:        "Fix bug /active",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantSection:  "Active",
		},
		{
			name:         "section with other metadata",
			input:        "Urgent fix !1 #backend /Pending",
			wantTitle:    "Urgent fix",
			wantPriority: 1,
			wantTags:     []string{"backend"},
			wantSection:  "Pending",
		},
		{
			name:        "invalid section",
			input:       "Fix bug /NonexistentSection",
			wantErr:     true,
			errContains: "section not found",
		},
		{
			name:         "slash in title without space should not trigger section",
			input:        "research/development task",
			wantTitle:    "research/development task",
			wantPriority: 3,
			wantSection:  "",
		},
		{
			name:         "slash in title with other metadata",
			input:        "word1/word2 !2 #backend",
			wantTitle:    "word1/word2",
			wantPriority: 2,
			wantTags:     []string{"backend"},
			wantSection:  "",
		},
		{
			name:         "space before slash triggers section",
			input:        "Fix bug /Active",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantSection:  "Active",
		},
		{
			name:         "embedded hash should not trigger tag",
			input:        "Fix C#code issue",
			wantTitle:    "Fix C#code issue",
			wantPriority: 3,
			wantTags:     nil,
		},
		{
			name:         "embedded exclamation should not trigger priority",
			input:        "Fix wow!1 bug",
			wantTitle:    "Fix wow!1 bug",
			wantPriority: 3,
		},
		{
			name:         "embedded at should not trigger date",
			input:        "Email user@tomorrow.com",
			wantTitle:    "Email user@tomorrow.com",
			wantPriority: 3,
		},
		{
			name:         "embedded tilde should not trigger recurrence",
			input:        "Path to~daily file",
			wantTitle:    "Path to~daily file",
			wantPriority: 3,
			wantHasRecur: false,
		},
		{
			name:         "embedded caret should not trigger type",
			input:        "Fix regex a^bug pattern",
			wantTitle:    "Fix regex a^bug pattern",
			wantPriority: 3,
			wantTaskType: models.TaskTypeTask,
		},
		{
			name:         "embedded dollar should not trigger status",
			input:        "Cost is 5$start today",
			wantTitle:    "Cost is 5$start today",
			wantPriority: 3,
			wantStatus:   "",
		},
		{
			name:         "embedded greater than should not trigger parent",
			input:        "Value a>1 check",
			wantTitle:    "Value a>1 check",
			wantPriority: 3,
			wantParentID: nil,
		},
		{
			name:         "task type bug",
			input:        "^bug Fix crash",
			wantTitle:    "Fix crash",
			wantPriority: 3,
			wantTaskType: models.TaskTypeBug,
		},
		{
			name:         "task type epic",
			input:        "^epic User management system",
			wantTitle:    "User management system",
			wantPriority: 3,
			wantTaskType: models.TaskTypeEpic,
		},
		{
			name:         "task type story",
			input:        "^story Login flow",
			wantTitle:    "Login flow",
			wantPriority: 3,
			wantTaskType: models.TaskTypeStory,
		},
		{
			name:         "task type idea",
			input:        "^idea Add dark mode",
			wantTitle:    "Add dark mode",
			wantPriority: 3,
			wantTaskType: models.TaskTypeIdea,
		},
		{
			name:         "task type case insensitive",
			input:        "^BUG Fix crash",
			wantTitle:    "Fix crash",
			wantPriority: 3,
			wantTaskType: models.TaskTypeBug,
		},
		{
			name:         "task type with other metadata",
			input:        "^bug Fix crash !1 #urgent /Active",
			wantTitle:    "Fix crash",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantTaskType: models.TaskTypeBug,
			wantSection:  "Active",
		},
		{
			name:         "recurrence daily",
			input:        "Standup ~daily",
			wantTitle:    "Standup",
			wantPriority: 3,
			wantHasRecur: true,
		},
		{
			name:         "recurrence weekly",
			input:        "Review ~weekly",
			wantTitle:    "Review",
			wantPriority: 3,
			wantHasRecur: true,
		},
		{
			name:         "recurrence monthly",
			input:        "Monthly report ~monthly",
			wantTitle:    "Monthly report",
			wantPriority: 3,
			wantHasRecur: true,
		},
		{
			name:         "recurrence biweekly",
			input:        "Payroll ~biweekly",
			wantTitle:    "Payroll",
			wantPriority: 3,
			wantHasRecur: true,
		},
		{
			name:         "recurrence Nd format",
			input:        "Check plants ~3d",
			wantTitle:    "Check plants",
			wantPriority: 3,
			wantHasRecur: true,
		},
		{
			name:         "recurrence Nw format",
			input:        "Team sync ~2w",
			wantTitle:    "Team sync",
			wantPriority: 3,
			wantHasRecur: true,
		},
		{
			name:           "recurrence with due date",
			input:          "Standup @tomorrow ~daily",
			wantTitle:      "Standup",
			wantPriority:   3,
			wantHasDueDate: true,
			wantHasRecur:   true,
		},
		{
			name:        "invalid recurrence pattern",
			input:       "Task ~notapattern",
			wantErr:     true,
			errContains: "invalid recurrence",
		},
		{
			name:         "status start",
			input:        "Fix bug $start",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantStatus:   models.StatusInProgress,
			wantActivate: true,
		},
		{
			name:         "status blocked",
			input:        "Waiting for review $blocked",
			wantTitle:    "Waiting for review",
			wantPriority: 3,
			wantStatus:   models.StatusBlocked,
		},
		{
			name:         "status done",
			input:        "Already finished $done",
			wantTitle:    "Already finished",
			wantPriority: 3,
			wantStatus:   models.StatusDone,
		},
		{
			name:         "status canceled",
			input:        "Abandoned task $canceled",
			wantTitle:    "Abandoned task",
			wantPriority: 3,
			wantStatus:   models.StatusCanceled,
		},
		{
			name:         "status case insensitive",
			input:        "Fix bug $START",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantStatus:   models.StatusInProgress,
			wantActivate: true,
		},
		{
			name:         "status with other metadata",
			input:        "Critical fix !1 #urgent $start",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantStatus:   models.StatusInProgress,
			wantActivate: true,
		},
		{
			name:        "invalid status",
			input:       "Fix bug $invalid",
			wantErr:     true,
			errContains: "invalid status",
		},
		{
			name:         "hyphenated tag",
			input:        "Fix bug #my-tag",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantTags:     []string{"my-tag"},
		},
		{
			name:         "dotted tag",
			input:        "Update #v2.0 release",
			wantTitle:    "Update release",
			wantPriority: 3,
			wantTags:     []string{"v2.0"},
		},
		{
			name:         "tag with multiple hyphens",
			input:        "Add feature #long-hyphen-tag",
			wantTitle:    "Add feature",
			wantPriority: 3,
			wantTags:     []string{"long-hyphen-tag"},
		},
		{
			name:         "status standby",
			input:        "Paused work $standby",
			wantTitle:    "Paused work",
			wantPriority: 3,
			wantStatus:   models.StatusStandby,
		},
		{
			name:         "status hold (alias for standby)",
			input:        "On hold $hold",
			wantTitle:    "On hold",
			wantPriority: 3,
			wantStatus:   models.StatusStandby,
		},
		{
			name:           "all features combined",
			input:          "^bug Critical fix !1 #urgent #production @tomorrow ~weekly /Active >1",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"urgent", "production"},
			wantHasDueDate: true,
			wantParentID:   strPtr("epic-1"),
			wantTaskType:   models.TaskTypeBug,
			wantSection:    "Active",
			wantHasRecur:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseQuickCapture(tt.input, project, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.title != tt.wantTitle {
				t.Errorf("title = %q, want %q", result.title, tt.wantTitle)
			}

			if result.priority != tt.wantPriority {
				t.Errorf("priority = %d, want %d", result.priority, tt.wantPriority)
			}

			if !slices.Equal(result.tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", result.tags, tt.wantTags)
			}

			hasDueDate := result.dueDate != nil
			if hasDueDate != tt.wantHasDueDate {
				t.Errorf("hasDueDate = %v, want %v", hasDueDate, tt.wantHasDueDate)
			}

			if tt.wantParentID != nil {
				if result.parentID == nil {
					t.Errorf("parentID = nil, want %q", *tt.wantParentID)
				} else if *result.parentID != *tt.wantParentID {
					t.Errorf("parentID = %q, want %q", *result.parentID, *tt.wantParentID)
				}
			} else if result.parentID != nil {
				t.Errorf("parentID = %q, want nil", *result.parentID)
			}

			if tt.wantTaskType != "" && result.taskType != tt.wantTaskType {
				t.Errorf("taskType = %q, want %q", result.taskType, tt.wantTaskType)
			}

			if tt.wantSection != "" && result.section != tt.wantSection {
				t.Errorf("section = %q, want %q", result.section, tt.wantSection)
			}

			hasRecur := result.recurrence != nil
			if hasRecur != tt.wantHasRecur {
				t.Errorf("hasRecurrence = %v, want %v", hasRecur, tt.wantHasRecur)
			}

			if tt.wantStatus != "" && result.status != tt.wantStatus {
				t.Errorf("status = %q, want %q", result.status, tt.wantStatus)
			}

			if result.activate != tt.wantActivate {
				t.Errorf("activate = %v, want %v", result.activate, tt.wantActivate)
			}
		})
	}
}
