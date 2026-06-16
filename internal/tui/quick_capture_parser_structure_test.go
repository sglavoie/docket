package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickCaptureStructureAndState(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
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
			input:        "^bug Fix crash !1 #urgent /Next",
			wantTitle:    "Fix crash",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantTaskType: models.TaskTypeBug,
			wantSection:  "Next",
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
			input:          "^bug Critical fix !1 #urgent #production @tomorrow ~weekly /Next >1",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"urgent", "production"},
			wantHasDueDate: true,
			wantParentID:   strPtr("epic-1"),
			wantTaskType:   models.TaskTypeBug,
			wantSection:    "Next",
			wantHasRecur:   true,
		},
	})
}
