package cmd

import (
	"testing"
)

func TestWorkflowToggle(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		setupWorkflowTest(t)

		err := runWorkflowToggle(workflowToggleCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects nonexistent rule ID", func(t *testing.T) {
		setupWorkflowTest(t)

		err := runWorkflowToggle(workflowToggleCmd, []string{"nonexistent-rule"})
		if err == nil {
			t.Error("expected error for nonexistent rule ID")
		}
	})
}
