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
		{WorkflowState: StateBacklog},
		{WorkflowState: StateDoing},
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
	task := &Task{WorkflowState: StateDoing}
	if !task.IsInState(StateDoing) {
		t.Errorf("in_progress task should report IsInState(StateDoing)")
	}
	if task.IsInState(StateBacklog) {
		t.Errorf("in_progress task should not report IsInState(StateBacklog)")
	}
}
