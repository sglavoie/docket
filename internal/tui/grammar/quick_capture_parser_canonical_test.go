package grammar

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickCaptureLegacyFormsRemainTitleText(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{name: "type word", input: "Fix crash type:bug", wantTitle: "Fix crash type:bug", wantPriority: 0},
		{name: "priority", input: "Fix crash priority:1", wantTitle: "Fix crash priority:1", wantPriority: 0},
		{name: "short priority", input: "Fix crash pri:1", wantTitle: "Fix crash pri:1", wantPriority: 0},
		{name: "tag", input: "Fix crash tag:backend", wantTitle: "Fix crash tag:backend", wantPriority: 0},
		{name: "due", input: "Review PR due:tomorrow", wantTitle: "Review PR due:tomorrow", wantPriority: 0},
		{name: "recurrence", input: "Review PR recur:weekly", wantTitle: "Review PR recur:weekly", wantPriority: 0},
		{name: "project", input: "Ship project:core", wantTitle: "Ship project:core", wantPriority: 0},
		{name: "short project", input: "Ship proj:core", wantTitle: "Ship proj:core", wantPriority: 0},
		{name: "parent", input: "Child parent:1", wantTitle: "Child parent:1", wantPriority: 0},
		{name: "state", input: "Ship state:ready", wantTitle: "Ship state:ready", wantPriority: 0},
	})
}

func TestParseQuickCaptureRetiredTaskTypeAndOrdinaryTags(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{name: "bug sigil retired", input: "^bug Fix crash", wantErr: true, errContains: "use #bug"},
		{name: "case insensitive retired", input: "Plan launch ^EPIC", wantErr: true, errContains: "task type"},
		{name: "legacy tag remains ordinary", input: "Fix crash #type:bug", wantTitle: "Fix crash", wantPriority: 0, wantTags: []string{"type-bug"}},
		{name: "unknown type", input: "Fix crash ^task", wantErr: true, errContains: "use #tag"},
	})
}

func TestParseQuickCaptureNamedPriorityValuesRemainTitleText(t *testing.T) {
	for _, value := range []string{"urgent", "high", "normal", "medium", "low", "h", "m", "l"} {
		t.Run(value, func(t *testing.T) {
			result, err := capturetext.ParseQuickCapture("Task !"+value, quickCaptureTestProject(), nil, false)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if result.Title != "Task !"+value || result.Priority != 0 || result.PrioritySet {
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
			wantPriority: 0,
			wantParentID: strPtr("epic-1"),
		},
		{
			name:         "parent by title prefix",
			input:        "Implementation detail >Login",
			wantTitle:    "Implementation detail",
			wantPriority: 0,
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
			input:        "Fix bug $doing",
			wantTitle:    "Fix bug",
			wantPriority: 0,
			wantState:    models.StateDoing,
		},
		{
			name:         "state waiting",
			input:        "Waiting for review $waiting",
			wantTitle:    "Waiting for review",
			wantPriority: 0,
			wantState:    models.StateWaiting,
		},
		{
			name:         "state done",
			input:        "Already finished $done",
			wantTitle:    "Already finished",
			wantPriority: 0,
			wantState:    models.StateDone,
		},
		{
			name:         "state canceled",
			input:        "Abandoned task $canceled",
			wantTitle:    "Abandoned task",
			wantPriority: 0,
			wantState:    models.StateCanceled,
		},
		{
			name:         "state with other metadata",
			input:        "Critical fix !1 #urgent $doing",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"urgent"},
			wantState:    models.StateDoing,
		},
		{
			name:        "retired hold alias rejected",
			input:       "On hold $hold",
			wantErr:     true,
			errContains: "use $paused",
		},
		{name: "today planning destination", input: "Plan work $today", wantTitle: "Plan work", wantDestination: app.CaptureDestinationToday},
		{name: "next planning destination", input: "Queue work $next", wantTitle: "Queue work", wantDestination: app.CaptureDestinationNext},
	})
}
