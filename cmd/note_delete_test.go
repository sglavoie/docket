package cmd

import (
	"testing"
)

func resetNoteDeleteVars() {
	noteDeleteForce = false
}

func TestRunNoteDelete(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetNoteDeleteVars()
		err := runNoteDelete(noteDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing note ID")
		}
	})
}
