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

	ready, _ := NewTask("ready", 1, TaskTypeTask, "Ready")
	started, _ := NewTask("started", 2, TaskTypeTask, "Started")
	next, _ := NewTask("next", 3, TaskTypeTask, "Next")
	started.ApplyWorkflowState(StateInProgress)
	next.ApplyWorkflowState(StateNext)
	project.Tasks = []*Task{ready, started, next}

	if got := project.GetTasksByState(StateReady); !reflect.DeepEqual(got, []*Task{ready}) {
		t.Fatalf("ready tasks = %#v, want only ready task", got)
	}
	if got := project.GetTasksByState(StateInProgress); !reflect.DeepEqual(got, []*Task{started}) {
		t.Fatalf("in-progress tasks = %#v, want only started task", got)
	}
	counts := project.CountTasksByState()
	if counts[StateReady] != 1 || counts[StateInProgress] != 1 || counts[StateNext] != 1 {
		t.Fatalf("state counts = %#v", counts)
	}
}

func TestAllModeWorkflowStateGroupsOrderAndSemantics(t *testing.T) {
	want := []WorkflowStateGroup{
		StateGroupToday,
		StateGroupInProgress,
		StateGroupInbox,
		StateGroupNext,
		StateGroupBacklog,
		StateGroupReview,
		StateGroupWaiting,
		StateGroupPaused,
		StateGroupSomeday,
	}
	if got := AllModeWorkflowStateGroups(); !slices.Equal(got, want) {
		t.Fatalf("All group order = %v, want %v", got, want)
	}

	for _, group := range AllWorkflowStateGroups() {
		if !group.IsValid() || group.Label() == "" || group.ThemeRole() == "" {
			t.Errorf("group %q has incomplete presentation semantics", group)
		}
		state, ok := group.WorkflowState()
		if group == StateGroupToday {
			if ok || !group.IsSynthetic() {
				t.Errorf("Today must be synthetic, got state %q ok=%v", state, ok)
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
