package models

import "testing"

func TestTaskIsTerminalState(t *testing.T) {
	terminalCases := []*Task{
		{WorkflowState: StateDone},
		{WorkflowState: StateCanceled},
	}
	for _, task := range terminalCases {
		if !task.IsTerminalState() {
			t.Errorf("task (state=%q) should be terminal", task.State())
		}
	}

	nonterminalCases := []*Task{
		{WorkflowState: StateReady},
		{WorkflowState: StateInProgress},
		{WorkflowState: StateReview},
		{WorkflowState: StateWaiting},
		{WorkflowState: StateInbox},
	}
	for _, task := range nonterminalCases {
		if task.IsTerminalState() {
			t.Errorf("task (state=%q) should not be terminal", task.State())
		}
	}
}

func TestTaskIsInState(t *testing.T) {
	task := &Task{WorkflowState: StateInProgress}
	if !task.IsInState(StateInProgress) {
		t.Errorf("in_progress task should report IsInState(StateInProgress)")
	}
	if task.IsInState(StateReady) {
		t.Errorf("in_progress task should not report IsInState(StateReady)")
	}
}

func TestTaskIsDependencyBlocked(t *testing.T) {
	project, err := NewProject("p1", "Proj", "")
	if err != nil {
		t.Fatal(err)
	}
	blocker, _ := NewTask("b", 1, TaskTypeTask, "Blocker")
	blocked, _ := NewTask("k", 2, TaskTypeTask, "Blocked")
	blocked.DependsOn = []string{"b"}
	project.Tasks = []*Task{blocker, blocked}

	if !blocked.IsDependencyBlocked(project) {
		t.Errorf("task with an incomplete dependency should be dependency-blocked")
	}
	// Dependency blocking is derived, not a state: the blocked task is still in
	// a nonterminal workflow state.
	if blocked.IsTerminalState() {
		t.Errorf("dependency-blocked task must not be terminal")
	}

	blocker.Complete()
	if blocked.IsDependencyBlocked(project) {
		t.Errorf("task should be unblocked once its dependency completes")
	}

	// Nil project is tolerated and reports not blocked.
	if blocked.IsDependencyBlocked(nil) {
		t.Errorf("nil project should yield not blocked")
	}
}
