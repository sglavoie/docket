package config

import "testing"

// Counting a press clones the map rather than writing through it, stops at the
// threshold, and the validated shape refuses a count past it.
func TestFooterKeyUseCountsToTheThresholdOnACopy(t *testing.T) {
	base := NewMachineState()
	base.LearnedFooterKeys = map[string]int{"b": FooterKeyLearnedUses - 1}

	next, moved := base.WithFooterKeyUse("b")
	if !moved || next.LearnedFooterKeys["b"] != FooterKeyLearnedUses {
		t.Fatalf("count = %d moved %v, want the threshold", next.LearnedFooterKeys["b"], moved)
	}
	if base.LearnedFooterKeys["b"] != FooterKeyLearnedUses-1 {
		t.Fatal("counting a press wrote through to the state it was copied from")
	}
	if !next.FooterKeyLearned("b") || base.FooterKeyLearned("b") {
		t.Fatal("learned answers disagree with the counts")
	}
	if _, moved := next.WithFooterKeyUse("b"); moved {
		t.Fatal("a learned key kept counting")
	}
	if fresh, moved := (MachineState{}).WithFooterKeyUse("a"); !moved || fresh.LearnedFooterKeys["a"] != 1 {
		t.Fatalf("a first press on an empty state = %v", fresh.LearnedFooterKeys)
	}

	for _, bad := range []map[string]int{{"": 1}, {"a": 0}, {"a": FooterKeyLearnedUses + 1}} {
		state := NewMachineState()
		state.LearnedFooterKeys = bad
		if err := state.Validate(); err == nil {
			t.Fatalf("learned footer keys %v validated", bad)
		}
	}
	clone := next.Clone()
	clone.LearnedFooterKeys["b"] = 1
	if next.LearnedFooterKeys["b"] != FooterKeyLearnedUses {
		t.Fatal("Clone shared the learned footer keys")
	}
}
