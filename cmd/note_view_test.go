package cmd

import (
	"testing"
)

func TestRunNoteView(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		err := runNoteView(noteViewCmd, []string{})
		if err == nil {
			t.Error("expected error for missing note ID")
		}
	})
}
