package tui

import "testing"

func TestParseQuickCaptureErrors(t *testing.T) {
	runQuickCaptureParseTests(t, []quickCaptureParseTestCase{
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
			name:        "invalid section",
			input:       "Fix bug /NonexistentSection",
			wantErr:     true,
			errContains: "section not found",
		},
		{
			name:        "invalid recurrence pattern",
			input:       "Task ~notapattern",
			wantErr:     true,
			errContains: "invalid recurrence",
		},
		{
			name:        "invalid status",
			input:       "Fix bug $invalid",
			wantErr:     true,
			errContains: "invalid status",
		},
	})
}