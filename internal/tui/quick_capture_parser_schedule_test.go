package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
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
		{
			name:        "recurrence daily retired",
			input:       "Standup ~daily",
			wantErr:     true,
			errContains: "use @date",
		},
		{
			name:        "recurrence weekly retired",
			input:       "Review ~weekly",
			wantErr:     true,
			errContains: "use @date",
		},
		{
			name:        "recurrence monthly retired",
			input:       "Monthly report ~monthly",
			wantErr:     true,
			errContains: "use @date",
		},
		{
			name:        "recurrence biweekly retired",
			input:       "Payroll ~biweekly",
			wantErr:     true,
			errContains: "use @date",
		},
		{
			name:        "recurrence Nd format retired",
			input:       "Check plants ~3d",
			wantErr:     true,
			errContains: "use @date",
		},
		{
			name:        "recurrence Nw format retired",
			input:       "Team sync ~2w",
			wantErr:     true,
			errContains: "use @date",
		},
		{
			name:        "recurrence with due date retired",
			input:       "Standup @tomorrow ~daily",
			wantErr:     true,
			errContains: "use @date",
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
