package cmd

import (
	"testing"
)

func TestWorkflowShow(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		setupWorkflowTest(t)

		err := runWorkflowShow(workflowShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects nonexistent rule ID", func(t *testing.T) {
		setupWorkflowTest(t)

		err := runWorkflowShow(workflowShowCmd, []string{"nonexistent-rule"})
		if err == nil {
			t.Error("expected error for nonexistent rule ID")
		}
	})
}
