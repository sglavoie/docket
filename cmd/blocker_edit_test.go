package cmd

import (
	"testing"
)

func resetBlockerEditVars() {
	blockerEditTitle = ""
	blockerEditDescription = ""
	blockerUnresolve = false
}

func TestRunBlockerEdit(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetBlockerEditVars()
		err := runBlockerEdit(blockerEditCmd, []string{})
		if err == nil {
			t.Error("expected error for missing blocker ID")
		}
	})
}
