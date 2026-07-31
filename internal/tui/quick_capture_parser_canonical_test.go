package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickCaptureLegacyFormsRemainTitleText(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{name: "type word", input: "Fix crash type:bug", wantTitle: "Fix crash type:bug", wantPriority: 3},
		{name: "priority", input: "Fix crash priority:1", wantTitle: "Fix crash priority:1", wantPriority: 3},
		{name: "short priority", input: "Fix crash pri:1", wantTitle: "Fix crash pri:1", wantPriority: 3},
		{name: "tag", input: "Fix crash tag:backend", wantTitle: "Fix crash tag:backend", wantPriority: 3},
		{name: "due", input: "Review PR due:tomorrow", wantTitle: "Review PR due:tomorrow", wantPriority: 3},
		{name: "recurrence", input: "Review PR recur:weekly", wantTitle: "Review PR recur:weekly", wantPriority: 3},
		{name: "project", input: "Ship project:core", wantTitle: "Ship project:core", wantPriority: 3},
		{name: "short project", input: "Ship proj:core", wantTitle: "Ship proj:core", wantPriority: 3},
		{name: "parent", input: "Child parent:1", wantTitle: "Child parent:1", wantPriority: 3},
		{name: "state", input: "Ship state:ready", wantTitle: "Ship state:ready", wantPriority: 3},
	})
}

func TestParseQuickCaptureSemanticTaskMarkers(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{name: "bug sigil", input: "^bug Fix crash", wantTitle: "Fix crash", wantPriority: 3, wantTags: []string{"bug"}},
		{name: "case insensitive", input: "Plan launch ^EPIC", wantTitle: "Plan launch", wantPriority: 3, wantTags: []string{"epic"}},
		{name: "legacy tag canonicalized", input: "Fix crash #type:bug", wantTitle: "Fix crash", wantPriority: 3, wantTags: []string{"bug"}},
		{name: "unknown type", input: "Fix crash ^task", wantErr: true, errContains: "invalid task type"},
		{name: "conflicting markers", input: "Fix crash #bug ^idea", wantErr: true, errContains: "only one type marker"},
	})
}

func TestParseQuickCaptureNamedPriorityValuesRemainTitleText(t *testing.T) {
	for _, value := range []string{"urgent", "high", "normal", "medium", "low", "h", "m", "l"} {
		t.Run(value, func(t *testing.T) {
			result, err := parseQuickCapture("Task !"+value, quickCaptureTestProject(), nil, false)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if result.title != "Task !"+value || result.priority != 3 || result.prioritySet {
				t.Fatalf("!%s parsed as priority: %+v", value, result)
			}
		})
	}
}

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
			name:         "state with other metadata",
			input:        "Critical fix !1 #urgent $in_progress",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantState:    models.StateInProgress,
		},
		{
			name:        "retired hold alias rejected",
			input:       "On hold $hold",
			wantErr:     true,
			errContains: "use $paused",
		},
		{
			name:           "all eight sigils combined",
			input:          "Critical fix ^bug !1 #production @tomorrow ~weekly $next >1",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"bug", "production"},
			wantHasDueDate: true,
			wantParentID:   strPtr("epic-1"),
			wantState:      models.StateNext,
			wantHasRecur:   true,
		},
	})
}
