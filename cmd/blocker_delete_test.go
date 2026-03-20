package cmd

import (
	"testing"
)

func TestRunBlockerDelete(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		err := runBlockerDelete(blockerDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing blocker ID")
		}
	})
}
