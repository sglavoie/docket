package status

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
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
			err:      storage.ErrNoUndoHistory,
			contains: []string{"Undo failed", "no undo history"},
		},
		{
			name:     "empty redo history",
			label:    "redo",
			err:      storage.ErrNoRedoHistory,
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

func TestExhaustedHistoryUsesErrorIdentity(t *testing.T) {
	for _, err := range []error{storage.ErrNoUndoHistory, fmt.Errorf("restore refused: %w", storage.ErrNoRedoHistory)} {
		if !IsExhaustedHistory(err) {
			t.Fatalf("did not recognize exhausted history: %v", err)
		}
	}
	for _, err := range []error{nil, errors.New("no undo history available"), errors.New("checking history available: disk failure")} {
		if IsExhaustedHistory(err) {
			t.Fatalf("classified unrelated failure as exhausted history: %v", err)
		}
	}
}
