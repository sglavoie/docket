package models

import (
	"reflect"
	"slices"
	"testing"
)

func TestProjectStateQueriesUseExactCanonicalState(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	backlog, _ := NewGenericTask("backlog", 1, "Backlog")
	doing, _ := NewGenericTask("doing", 2, "Doing")
	next, _ := NewGenericTask("next", 3, "Next")
	doing.ApplyWorkflowState(StateDoing)
	next.ApplyWorkflowState(StateWaiting)
	project.Tasks = []*Task{backlog, doing, next}

	if got := project.GetTasksByState(StateBacklog); !reflect.DeepEqual(got, []*Task{backlog}) {
		t.Fatalf("backlog tasks = %#v, want only backlog task", got)
	}
	if got := project.GetTasksByState(StateDoing); !reflect.DeepEqual(got, []*Task{doing}) {
		t.Fatalf("doing tasks = %#v, want only doing task", got)
	}
	counts := project.CountTasksByState()
	if counts[StateBacklog] != 1 || counts[StateDoing] != 1 || counts[StateWaiting] != 1 {
		t.Fatalf("state counts = %#v", counts)
	}
}

func TestAllModeWorkflowStateGroupsOrderAndSemantics(t *testing.T) {
	want := []WorkflowStateGroup{
		StateGroupToday,
		StateGroupDoing,
		StateGroupInbox,
		StateGroupNext,
		StateGroupPaused,
		StateGroupBacklog,
		StateGroupWaiting,
		StateGroupSomeday,
	}
	if got := AllModeWorkflowStateGroups(); !slices.Equal(got, want) {
		t.Fatalf("All group order = %v, want %v", got, want)
	}

	for _, group := range append(AllModeWorkflowStateGroups(), StateGroupDone, StateGroupCanceled) {
		if !group.IsValid() || group.Label() == "" || group.ThemeRole() == "" {
			t.Errorf("group %q has incomplete presentation semantics", group)
		}
		state, ok := group.WorkflowState()
		if group == StateGroupToday || group == StateGroupNext {
			if ok || !group.IsSynthetic() {
				t.Errorf("%s must be synthetic, got state %q ok=%v", group, state, ok)
			}
			continue
		}
		if !ok || WorkflowStateGroupForState(state) != group {
			t.Errorf("group/state round trip for %q = %q, %v", group, state, ok)
		}
	}
}

func TestWorkflowStateGroupsCoverEveryCanonicalState(t *testing.T) {
	seen := make(map[WorkflowStateGroup]bool)
	for _, state := range AllWorkflowStates() {
		group := WorkflowStateGroupForState(state)
		if !group.IsValid() || group.IsSynthetic() {
			t.Errorf("state %q maps to invalid group %q", state, group)
		}
		if seen[group] {
			t.Errorf("state group %q is not one-to-one", group)
		}
		seen[group] = true
	}
}
