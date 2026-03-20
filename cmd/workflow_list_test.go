package cmd

import (
	"testing"
)

func TestWorkflowList(t *testing.T) {
	t.Run("runs without error when no rules configured", func(t *testing.T) {
		setupWorkflowTest(t)
		defer func() { workflowListJSON = false }()

		err := runWorkflowList(workflowListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("runs without error with JSON flag", func(t *testing.T) {
		setupWorkflowTest(t)
		defer func() { workflowListJSON = false }()

		workflowListJSON = true
		err := runWorkflowList(workflowListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
