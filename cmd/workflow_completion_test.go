package cmd

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func workflowFlagCompletions(t *testing.T, command *cobra.Command, flag string) []string {
	t.Helper()
	fn, ok := command.GetFlagCompletionFunc(flag)
	if !ok {
		t.Fatalf("%s has no completion metadata for --%s", command.Name(), flag)
	}
	values, _ := fn(command, nil, "")
	return values
}

func TestWorkflowCanonicalCompletionMetadata(t *testing.T) {
	fields := workflowFlagCompletions(t, workflowAddCmd, "field")
	for _, want := range []string{"state", "state_changed", "parent_state", "all_tasks_state", "majority_task_state", "has_tasks_with_state"} {
		if !slices.Contains(fields, want) {
			t.Errorf("field completions missing %q", want)
		}
	}
	for _, legacy := range []string{"status", "section", "status_changed", "section_changed", "parent_status"} {
		if slices.Contains(fields, legacy) {
			t.Errorf("field completions expose legacy value %q", legacy)
		}
	}

	operations := workflowFlagCompletions(t, workflowAddCmd, "op")
	for _, want := range []string{"in", "not_in"} {
		if !slices.Contains(operations, want) {
			t.Errorf("operation completions missing %q", want)
		}
	}

	actions := workflowFlagCompletions(t, workflowAddCmd, "action")
	if !slices.Contains(actions, "set_state") {
		t.Error("action completions missing set_state")
	}
	if slices.Contains(actions, "set_status") || slices.Contains(actions, "set_section") {
		t.Error("action completions expose legacy lifecycle actions")
	}

	states := workflowFlagCompletions(t, workflowAddCmd, "scope-state")
	for _, want := range stateCompletions {
		if !slices.Contains(states, want) {
			t.Errorf("scope-state completions missing %q", want)
		}
	}
}

func TestWorkflowFlagBuildersRejectLegacyLifecycleVocabulary(t *testing.T) {
	if _, err := buildTriggersFromFlags(
		[]string{"state"},
		[]string{"in"},
		[]string{"ready,next"},
	); err != nil {
		t.Fatalf("canonical state trigger rejected: %v", err)
	}
	if _, err := buildTriggersFromFlags(
		[]string{"status"},
		[]string{"equals"},
		[]string{"todo"},
	); err == nil {
		t.Error("legacy status trigger should not be authorable")
	}

	if _, err := buildActionFromFlags("set_state", "done"); err != nil {
		t.Fatalf("canonical set_state action rejected: %v", err)
	}
	if _, err := buildActionFromFlags("set_section", "Completed"); err == nil {
		t.Error("legacy set_section action should not be authorable")
	}
}
