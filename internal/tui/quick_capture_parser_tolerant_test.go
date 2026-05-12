package tui

import "testing"

func TestParseQuickCapturePriorityAliases(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{
			name:         "!urgent maps to priority 1",
			input:        "Fix crash !urgent",
			wantTitle:    "Fix crash",
			wantPriority: 1,
		},
		{
			name:         "!high maps to priority 1",
			input:        "Fix crash !high",
			wantTitle:    "Fix crash",
			wantPriority: 1,
		},
		{
			name:         "!critical maps to priority 1",
			input:        "Fix crash !critical",
			wantTitle:    "Fix crash",
			wantPriority: 1,
		},
		{
			name:         "!normal maps to priority 3",
			input:        "Routine task !normal",
			wantTitle:    "Routine task",
			wantPriority: 3,
		},
		{
			name:         "!medium maps to priority 3",
			input:        "Medium task !medium",
			wantTitle:    "Medium task",
			wantPriority: 3,
		},
		{
			name:         "!low maps to priority 5",
			input:        "Low-priority task !low",
			wantTitle:    "Low-priority task",
			wantPriority: 5,
		},
		{
			name:         "!lowest maps to priority 5",
			input:        "Backlog item !lowest",
			wantTitle:    "Backlog item",
			wantPriority: 5,
		},
		{
			name:         "priority word alias case-insensitive",
			input:        "Fix crash !URGENT",
			wantTitle:    "Fix crash",
			wantPriority: 1,
		},
		{
			name:         "priority word alias at start",
			input:        "!high Fix crash",
			wantTitle:    "Fix crash",
			wantPriority: 1,
		},
		{
			name:         "numeric priority still works",
			input:        "Fix crash !2",
			wantTitle:    "Fix crash",
			wantPriority: 2,
		},
	})
}

func TestParseQuickCaptureDueColon(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
		{
			name:           "due:tomorrow",
			input:          "Review PR due:tomorrow",
			wantTitle:      "Review PR",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due:friday",
			input:          "Team meeting due:friday",
			wantTitle:      "Team meeting",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due:3d bare shorthand",
			input:          "Submit report due:3d",
			wantTitle:      "Submit report",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due:2w bare shorthand",
			input:          "Quarterly review due:2w",
			wantTitle:      "Quarterly review",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due:1m bare shorthand",
			input:          "Monthly report due:1m",
			wantTitle:      "Monthly report",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due:next week multi-word",
			input:          "Planning due:next week",
			wantTitle:      "Planning",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due:next friday multi-word",
			input:          "Demo prep due:next friday",
			wantTitle:      "Demo prep",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due: at start of input",
			input:          "due:tomorrow Fix a bug",
			wantTitle:      "Fix a bug",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "due: combined with priority alias and tag",
			input:          "Critical fix !urgent #backend due:tomorrow",
			wantTitle:      "Critical fix",
			wantPriority:   1,
			wantTags:       []string{"backend"},
			wantHasDueDate: true,
		},
		{
			name:        "invalid due: date returns error",
			input:       "Fix bug due:notadate",
			wantErr:     true,
			errContains: "invalid date",
		},
		{
			name:           "@date takes precedence over due: (@ parsed first)",
			input:          "Fix bug @tomorrow due:friday",
			wantTitle:      "Fix bug",
			wantPriority:   3,
			wantHasDueDate: true,
		},
	})
}
