package cmd

import (
	"testing"
)

func TestWorkflowReset(t *testing.T) {
	t.Run("rejects no args and no --all", func(t *testing.T) {
		setupWorkflowTest(t)
		defer func() { workflowResetAll = false }()

		err := runWorkflowReset(workflowResetCmd, []string{})
		if err == nil {
			t.Error("expected error when neither --all nor a rule ID is provided")
		}
	})

	t.Run("rejects --all combined with a rule ID", func(t *testing.T) {
		setupWorkflowTest(t)
		defer func() { workflowResetAll = false }()

		workflowResetAll = true
		err := runWorkflowReset(workflowResetCmd, []string{"some-rule-id"})
		if err == nil {
			t.Error("expected error when --all is combined with a positional rule ID")
		}
	})
}
