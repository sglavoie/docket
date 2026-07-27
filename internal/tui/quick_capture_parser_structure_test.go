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
			input:        "^bug Fix crash !1 #urgent $next",
			wantTitle:    "Fix crash",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantTaskType: models.TaskTypeBug,
			wantState:    models.StateNext,
		},
		{
			name:         "state in progress",
			input:        "Fix bug $in_progress",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantState:    models.StateInProgress,
		},
		{
			name:         "state waiting",
			input:        "Waiting for review $waiting",
			wantTitle:    "Waiting for review",
			wantPriority: 3,
			wantState:    models.StateWaiting,
		},
		{
			name:         "state done",
			input:        "Already finished $done",
			wantTitle:    "Already finished",
			wantPriority: 3,
			wantState:    models.StateDone,
		},
		{
			name:         "state canceled",
			input:        "Abandoned task $canceled",
			wantTitle:    "Abandoned task",
			wantPriority: 3,
			wantState:    models.StateCanceled,
		},
		{
			name:         "state case insensitive",
			input:        "Fix bug $IN_PROGRESS",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantState:    models.StateInProgress,
		},
		{
			name:         "state with other metadata",
			input:        "Critical fix !1 #urgent $in_progress",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantState:    models.StateInProgress,
		},
		{
			name:         "state paused",
			input:        "Paused work $paused",
			wantTitle:    "Paused work",
			wantPriority: 3,
			wantState:    models.StatePaused,
		},
		{
			name:        "retired hold alias rejected",
			input:       "On hold $hold",
			wantErr:     true,
			errContains: "use $paused",
		},
		{
			name:           "all features combined",
			input:          "^bug Critical fix !1 #urgent #production @tomorrow ~weekly $next >1",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"urgent", "production"},
			wantHasDueDate: true,
			wantParentID:   strPtr("epic-1"),
			wantTaskType:   models.TaskTypeBug,
			wantState:      models.StateNext,
			wantHasRecur:   true,
		},
	})
}
