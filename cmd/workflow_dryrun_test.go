package cmd

import (
	"testing"
)

func TestWorkflowDryRun(t *testing.T) {
	t.Run("runs without error when no rules configured", func(t *testing.T) {
		setupWorkflowTest(t)

		err := runWorkflowTest(workflowTestCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
