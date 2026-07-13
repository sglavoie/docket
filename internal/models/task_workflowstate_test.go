package models

import "testing"

// TestDeriveWorkflowStateMappingMatrix pins the exact legacy -> WorkflowState
// precedence table from context.md, including the priority ordering between
// competing legacy fields (earlier rules win).
func TestDeriveWorkflowStateMappingMatrix(t *testing.T) {
	tests := []struct {
		name    string
		status  TaskStatus
		section string
		stage   string
		want    WorkflowState
	}{
		// Rule 1: canceled status wins over everything else.
		{"canceled status", StatusCanceled, "", "", StateCanceled},
		{"canceled beats completed section", StatusCanceled, SectionCompleted, "", StateCanceled},
		{"canceled beats review section", StatusCanceled, SectionReview, "Review", StateCanceled},

		// Rule 2: done status or completed section -> done.
		{"done status", StatusDone, "", "", StateDone},
		{"completed section without done status", StatusTodo, SectionCompleted, "", StateDone},
		{"done beats review section", StatusDone, SectionReview, "", StateDone},
		{"completed section beats in_progress status", StatusInProgress, SectionCompleted, "", StateDone},

		// Rule 3: review status/section/stage -> review.
		{"review status", StatusReview, "", "", StateReview},
		{"review section", StatusTodo, SectionReview, "", StateReview},
		{"review stage only", StatusTodo, "", "Review", StateReview},
		{"review stage case-insensitive", StatusTodo, "", "review", StateReview},
		{"review beats next section", StatusReview, SectionNext, "", StateReview},

		// Rule 4: in_progress status -> in_progress.
		{"in_progress status", StatusInProgress, "", "", StateInProgress},
		{"in_progress beats waiting section", StatusInProgress, SectionWaiting, "", StateInProgress},

		// Rule 5: blocked status or waiting section -> waiting.
		{"blocked status", StatusBlocked, "", "", StateWaiting},
		{"waiting section without blocked status", StatusTodo, SectionWaiting, "", StateWaiting},

		// Rule 6: standby status or standby section -> paused.
		{"standby status", StatusStandby, "", "", StatePaused},
		{"standby section", StatusTodo, SectionStandby, "", StatePaused},

		// Rule 7: maybe status or someday section -> someday.
		{"maybe status", StatusMaybe, "", "", StateSomeday},
		{"someday section", StatusTodo, SectionSomeday, "", StateSomeday},

		// Rule 8: inbox section -> inbox.
		{"inbox section", StatusTodo, SectionInbox, "", StateInbox},

		// Rule 9: next section -> next.
		{"next section", StatusTodo, SectionNext, "", StateNext},

		// Rule 10: everything else -> ready.
		{"todo no section", StatusTodo, "", "", StateReady},
		{"todo with non-review stage", StatusTodo, "", "Design", StateReady},
		{"empty everything", "", "", "", StateReady},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveWorkflowState(tt.status, tt.section, tt.stage)
			if got != tt.want {
				t.Errorf("DeriveWorkflowState(%q, %q, %q) = %q, want %q",
					tt.status, tt.section, tt.stage, got, tt.want)
			}
			// A task carrying the derived state must report that same state.
			task := &Task{WorkflowState: tt.want}
			if task.State() != tt.want {
				t.Errorf("Task.State() = %q, want %q", task.State(), tt.want)
			}
		})
	}
}

// TestDeriveWorkflowStateAlwaysValid confirms the mapping is total: every legacy
// status paired with every legacy section resolves to a valid canonical state.
func TestDeriveWorkflowStateAlwaysValid(t *testing.T) {
	statuses := append(AllTaskStatuses(), StatusStandby, "", "garbage")
	sections := []string{
		"", SectionInbox, SectionNext, SectionWaiting, SectionSomeday,
		SectionReview, SectionStandby, SectionCompleted, "CustomGroup",
	}
	stages := []string{"", "Review", "Design", "Implement"}

	for _, st := range statuses {
		for _, sec := range sections {
			for _, stg := range stages {
				got := DeriveWorkflowState(st, sec, stg)
				if !got.IsValid() {
					t.Errorf("DeriveWorkflowState(%q, %q, %q) = %q (invalid)", st, sec, stg, got)
				}
			}
		}
	}
}

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
