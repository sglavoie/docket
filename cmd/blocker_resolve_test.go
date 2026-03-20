package cmd

import (
	"testing"
)

func TestRunBlockerResolve(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		err := runBlockerResolve(blockerResolveCmd, []string{})
		if err == nil {
			t.Error("expected error for missing blocker ID")
		}
	})
}
