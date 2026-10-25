package grammar

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func TestParseQuickCaptureScheduleAndPlacement(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{
			name:           "with due date tomorrow",
			input:          "Review PR @tomorrow",
			wantTitle:      "Review PR",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date relative days",
			input:          "Submit report @+3d",
			wantTitle:      "Submit report",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date weekday",
			input:          "Team meeting @friday",
			wantTitle:      "Team meeting",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date exact",
			input:          "Project deadline @2025-12-31",
			wantTitle:      "Project deadline",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date in 2 weeks",
			input:          "Review code @in 2 weeks",
			wantTitle:      "Review code",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date in 3 days",
			input:          "Submit report @in 3 days",
			wantTitle:      "Submit report",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date next monday",
			input:          "Team sync @next monday",
			wantTitle:      "Team sync",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date next friday",
			input:          "Demo prep @next friday",
			wantTitle:      "Demo prep",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		{
			name:           "with due date next week",
			input:          "Planning @next week",
			wantTitle:      "Planning",
			wantPriority:   0,
			wantHasDueDate: true,
		},
		// The tilde now belongs to dependencies, so the retired recurrence
		// vocabulary is no longer special-cased: each of these values is simply a
		// task reference the project cannot resolve.
		{
			name:        "former daily recurrence is an unresolved reference",
			input:       "Standup ~daily",
			wantErr:     true,
			errContains: "dependency not found",
		},
		{
			name:        "former weekly recurrence is an unresolved reference",
			input:       "Review ~weekly",
			wantErr:     true,
			errContains: "dependency not found",
		},
		{
			name:        "former Nd recurrence is an unresolved reference",
			input:       "Check plants ~3d",
			wantErr:     true,
			errContains: "dependency not found",
		},
		{
			name:        "former Nw recurrence is an unresolved reference",
			input:       "Team sync ~2w",
			wantErr:     true,
			errContains: "dependency not found",
		},
		{
			name:        "an unresolved reference beside a due date still refuses",
			input:       "Standup @tomorrow ~daily",
			wantErr:     true,
			errContains: "dependency not found",
		},
		{
			name:         "state assignment",
			input:        "Fix bug $backlog",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantState:    models.StateBacklog,
		},
		{
			name:         "state case insensitive",
			input:        "Fix bug $BACKLOG",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantState:    models.StateBacklog,
		},
		{
			name:         "state with other metadata",
			input:        "Urgent fix !1 #backend $waiting",
			wantTitle:    "Urgent fix",
			wantPriority: 1,
			wantTags:     []string{"backend"},
			wantState:    models.StateWaiting,
		},
		{
			name:           "multi-word date with other metadata",
			input:          "Critical fix !1 #urgent @in 2 weeks $backlog",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"urgent"},
			wantHasDueDate: true,
			wantState:      models.StateBacklog,
		},
		{
			name:           "all metadata combined",
			input:          "Urgent task !1 #backend #critical @tomorrow",
			wantTitle:      "Urgent task",
			wantPriority:   1,
			wantTags:       []string{"backend", "critical"},
			wantHasDueDate: true,
		},
	})
}
