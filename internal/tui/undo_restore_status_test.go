package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatUndoRedoRestoreErrorMessageExplainsFailureKind(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		err      error
		contains []string
	}{
		{
			name:     "empty undo history",
			label:    "undo",
			err:      errors.New("no undo history available"),
			contains: []string{"Undo failed", "no undo history"},
		},
		{
			name:     "empty redo history",
			label:    "redo",
			err:      errors.New("no redo history available"),
			contains: []string{"Redo failed", "no redo history"},
		},
		{
			name:     "grouped history diverged",
			label:    "undo",
			err:      errors.New("undo group \"g\" diverged for project \"Beta\": top entry group id \"other\" does not match \"g\""),
			contains: []string{"Undo failed", "grouped history diverged", "restore aborted"},
		},
		{
			name:     "partially unavailable history",
			label:    "redo",
			err:      errors.New("failed to read redo snapshot 2 for project \"Beta\": open .undo/redo_0002.md: no such file or directory"),
			contains: []string{"Redo failed", "redo history is partially unavailable", "restore aborted"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := formatUndoRedoRestoreErrorMessage(tt.label, tt.err)
			for _, want := range tt.contains {
				if !strings.Contains(msg, want) {
					t.Fatalf("message %q missing %q", msg, want)
				}
			}
		})
	}
}
