package cmd

import (
	"testing"
)

func resetNoteEditVars() {
	noteEditEditor = ""
}

func TestRunNoteEdit(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetNoteEditVars()
		err := runNoteEdit(noteEditCmd, []string{})
		if err == nil {
			t.Error("expected error for missing note ID")
		}
	})
}
