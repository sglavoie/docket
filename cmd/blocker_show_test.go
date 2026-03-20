package cmd

import (
	"testing"
)

func TestRunBlockerShow(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		err := runBlockerShow(blockerShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing blocker ID")
		}
	})
}
