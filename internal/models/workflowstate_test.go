package models

import "testing"

func TestAllWorkflowStatesAreValidAndComplete(t *testing.T) {
	all := AllWorkflowStates()
	if len(all) != 10 {
		t.Fatalf("AllWorkflowStates() len = %d, want 10", len(all))
	}
	seen := make(map[WorkflowState]bool, len(all))
	for _, s := range all {
		if !s.IsValid() {
			t.Errorf("state %q returned by AllWorkflowStates is not valid", s)
		}
		if seen[s] {
			t.Errorf("state %q appears more than once in AllWorkflowStates", s)
		}
		seen[s] = true
	}
}

func TestWorkflowStateIsValid(t *testing.T) {
	for _, s := range AllWorkflowStates() {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, bad := range []WorkflowState{"", "todo", "in-progress", "Done", "blocked", "unknown"} {
		if bad.IsValid() {
			t.Errorf("%q should not be valid", bad)
		}
	}
}

func TestWorkflowStateTerminalClassification(t *testing.T) {
	terminal := map[WorkflowState]bool{StateDone: true, StateCanceled: true}
	for _, s := range AllWorkflowStates() {
		wantTerminal := terminal[s]
		if s.IsTerminal() != wantTerminal {
			t.Errorf("%q IsTerminal() = %v, want %v", s, s.IsTerminal(), wantTerminal)
		}
		if s.IsNonterminal() == wantTerminal {
			t.Errorf("%q IsNonterminal() = %v, want %v", s, s.IsNonterminal(), !wantTerminal)
		}
	}
	// An invalid state is neither terminal nor a valid nonterminal state.
	if WorkflowState("bogus").IsNonterminal() {
		t.Errorf("invalid state should not report IsNonterminal")
	}
}

func TestParseWorkflowState(t *testing.T) {
	for _, s := range AllWorkflowStates() {
		got, ok := ParseWorkflowState(string(s))
		if !ok || got != s {
			t.Errorf("ParseWorkflowState(%q) = (%q, %v), want (%q, true)", s, got, ok, s)
		}
	}
	if got, ok := ParseWorkflowState("nope"); ok {
		t.Errorf("ParseWorkflowState(\"nope\") = (%q, %v), want ok=false", got, ok)
	}
	if StateInbox.String() != "inbox" {
		t.Errorf("StateInbox.String() = %q, want inbox", StateInbox.String())
	}
}

// TestCanTransitionStateMatrix exhaustively verifies every ordered pair of
// canonical states against the locked transition policy:
//   - self-transitions are rejected,
//   - nonterminal -> any other valid state is allowed,
//   - terminal -> only StateInbox (reopen) is allowed.
func TestCanTransitionStateMatrix(t *testing.T) {
	states := AllWorkflowStates()
	for _, from := range states {
		for _, to := range states {
			allowed := CanTransitionState(from, to) == ""

			var want bool
			switch {
			case from == to:
				want = false
			case from.IsTerminal():
				want = to == ReopenTargetState
			default:
				want = true
			}

			if allowed != want {
				t.Errorf("CanTransitionState(%q, %q) allowed = %v, want %v", from, to, allowed, want)
			}
		}
	}
}

func TestCanTransitionStateRejectsInvalid(t *testing.T) {
	if CanTransitionState("bogus", StateReady) == "" {
		t.Errorf("transition from invalid state should be rejected")
	}
	if CanTransitionState(StateReady, "bogus") == "" {
		t.Errorf("transition to invalid state should be rejected")
	}
}

func TestReopenTargetIsInbox(t *testing.T) {
	if ReopenTargetState != StateInbox {
		t.Fatalf("ReopenTargetState = %q, want inbox", ReopenTargetState)
	}
	for _, terminal := range []WorkflowState{StateDone, StateCanceled} {
		if CanTransitionState(terminal, StateInbox) != "" {
			t.Errorf("reopen (%q -> inbox) should be allowed", terminal)
		}
		for _, to := range AllWorkflowStates() {
			if to == StateInbox {
				continue
			}
			if CanTransitionState(terminal, to) == "" {
				t.Errorf("%q -> %q should be rejected (only reopen to inbox is allowed)", terminal, to)
			}
		}
	}
}
