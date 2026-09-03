package grammar

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
			name:        "dependency reference not found",
			input:       "Task ~weekly",
			wantErr:     true,
			errContains: "dependency not found",
		},
		{
			name:        "invalid destination",
			input:       "Fix bug $invalid",
			wantErr:     true,
			errContains: "invalid destination",
		},
		{name: "retired type", input: "Fix bug ^bug", wantErr: true, errContains: "use #bug"},
		{name: "retired priority four", input: "Fix bug !4", wantErr: true, errContains: "use !1, !2, or !3"},
		{name: "retired priority five", input: "Fix bug !5", wantErr: true, errContains: "use !1, !2, or !3"},
		{name: "retired ready destination", input: "Fix bug $ready", wantErr: true, errContains: "use $backlog"},
		{name: "retired in progress destination", input: "Fix bug $in_progress", wantErr: true, errContains: "use $doing"},
	})
}
