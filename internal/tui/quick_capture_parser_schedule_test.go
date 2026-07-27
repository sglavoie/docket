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
			name:         "state assignment",
			input:        "Fix bug $next",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantState:    models.StateNext,
		},
		{
			name:         "state case insensitive",
			input:        "Fix bug $NEXT",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantState:    models.StateNext,
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
			input:          "Critical fix !1 #urgent @in 2 weeks $next",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"urgent"},
			wantHasDueDate: true,
			wantState:      models.StateNext,
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
