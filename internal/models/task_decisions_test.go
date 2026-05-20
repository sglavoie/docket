package models

import "testing"

func TestAddDecision(t *testing.T) {
	t.Run("adds decision with all fields", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("Use PostgreSQL", "Better for complex queries", DecisionCategoryDesign)

		if len(task.Decisions) != 1 {
			t.Fatalf("got %d decisions, want 1", len(task.Decisions))
		}
		d := task.Decisions[0]
		if d.Text != "Use PostgreSQL" {
			t.Errorf("Text = %q, want %q", d.Text, "Use PostgreSQL")
		}
		if d.Rationale != "Better for complex queries" {
			t.Errorf("Rationale = %q, want %q", d.Rationale, "Better for complex queries")
		}
		if d.Category != DecisionCategoryDesign {
			t.Errorf("Category = %q, want %q", d.Category, DecisionCategoryDesign)
		}
		if d.Resolved {
			t.Error("new decision should not be resolved")
		}
	})
}

func TestRemoveDecision(t *testing.T) {
	t.Run("removes decision at valid index", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("First", "", DecisionCategoryDesign)
		task.AddDecision("Second", "", DecisionCategoryScope)

		ok := task.RemoveDecision(0)
		if !ok {
			t.Fatal("RemoveDecision returned false for valid index")
		}
		if len(task.Decisions) != 1 {
			t.Fatalf("got %d decisions, want 1", len(task.Decisions))
		}
		if task.Decisions[0].Text != "Second" {
			t.Errorf("remaining decision Text = %q, want %q", task.Decisions[0].Text, "Second")
		}
	})

	t.Run("out of range returns false", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("Only", "", DecisionCategoryUX)

		if task.RemoveDecision(-1) {
			t.Error("RemoveDecision should return false for negative index")
		}
		if task.RemoveDecision(1) {
			t.Error("RemoveDecision should return false for out-of-range index")
		}
		if task.RemoveDecision(99) {
			t.Error("RemoveDecision should return false for far out-of-range index")
		}
	})
}

func TestClearDecisions(t *testing.T) {
	t.Run("removes all decisions", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("First", "", DecisionCategoryDesign)
		task.AddDecision("Second", "", DecisionCategoryArchitecture)

		task.ClearDecisions()
		if len(task.Decisions) != 0 {
			t.Errorf("got %d decisions after clear, want 0", len(task.Decisions))
		}
	})

	t.Run("no-op on empty", func(t *testing.T) {
		task := &Task{}
		task.ClearDecisions()
		if len(task.Decisions) != 0 {
			t.Errorf("got %d decisions, want 0", len(task.Decisions))
		}
	})
}

func TestToggleDecisionResolved(t *testing.T) {
	t.Run("toggles resolved state", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("First", "", DecisionCategoryDesign)
		task.AddDecision("Second", "", DecisionCategoryScope)

		if task.Decisions[0].Resolved {
			t.Fatal("new decision should not be resolved")
		}

		ok := task.ToggleDecisionResolved(0)
		if !ok {
			t.Fatal("ToggleDecisionResolved returned false for valid index")
		}
		if !task.Decisions[0].Resolved {
			t.Error("decision should be resolved after toggle")
		}

		ok = task.ToggleDecisionResolved(0)
		if !ok {
			t.Fatal("ToggleDecisionResolved returned false for valid index")
		}
		if task.Decisions[0].Resolved {
			t.Error("decision should be unresolved after second toggle")
		}
	})

	t.Run("out of range returns false", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("Only", "", DecisionCategoryUX)

		if task.ToggleDecisionResolved(-1) {
			t.Error("ToggleDecisionResolved should return false for negative index")
		}
		if task.ToggleDecisionResolved(1) {
			t.Error("ToggleDecisionResolved should return false for out-of-range index")
		}
		if task.ToggleDecisionResolved(99) {
			t.Error("ToggleDecisionResolved should return false for far out-of-range index")
		}
	})

	t.Run("empty decisions returns false", func(t *testing.T) {
		task := &Task{}
		if task.ToggleDecisionResolved(0) {
			t.Error("ToggleDecisionResolved should return false for empty decisions")
		}
	})
}

func TestUnresolvedDecisionCount(t *testing.T) {
	t.Run("all unresolved", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("First", "", DecisionCategoryDesign)
		task.AddDecision("Second", "", DecisionCategoryScope)
		task.AddDecision("Third", "", DecisionCategoryUX)

		if got := task.UnresolvedDecisionCount(); got != 3 {
			t.Errorf("UnresolvedDecisionCount() = %d, want 3", got)
		}
	})

	t.Run("some resolved", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("First", "", DecisionCategoryDesign)
		task.AddDecision("Second", "", DecisionCategoryScope)
		task.AddDecision("Third", "", DecisionCategoryUX)
		task.Decisions[1].Resolved = true

		if got := task.UnresolvedDecisionCount(); got != 2 {
			t.Errorf("UnresolvedDecisionCount() = %d, want 2", got)
		}
	})

	t.Run("all resolved", func(t *testing.T) {
		task := &Task{}
		task.AddDecision("First", "", DecisionCategoryDesign)
		task.AddDecision("Second", "", DecisionCategoryScope)
		task.Decisions[0].Resolved = true
		task.Decisions[1].Resolved = true

		if got := task.UnresolvedDecisionCount(); got != 0 {
			t.Errorf("UnresolvedDecisionCount() = %d, want 0", got)
		}
	})

	t.Run("no decisions", func(t *testing.T) {
		task := &Task{}
		if got := task.UnresolvedDecisionCount(); got != 0 {
			t.Errorf("UnresolvedDecisionCount() = %d, want 0", got)
		}
	})
}

func TestValidDecisionCategories(t *testing.T) {
	cats := ValidDecisionCategories()
	if len(cats) != 7 {
		t.Errorf("got %d categories, want 7", len(cats))
	}
}

func TestIsValidDecisionCategory(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"design", true},
		{"architecture", true},
		{"ux", true},
		{"trade-off", true},
		{"scope", true},
		{"performance", true},
		{"security", true},
		{"unknown", false},
		{"", false},
		{"DESIGN", false},
	}

	for _, tc := range tests {
		got := IsValidDecisionCategory(tc.input)
		if got != tc.valid {
			t.Errorf("IsValidDecisionCategory(%q) = %v, want %v", tc.input, got, tc.valid)
		}
	}
}

// TestAddDecisionDropsEmptyText verifies that AddDecision rejects
// empty/whitespace-only text. Such decisions cannot round-trip through the
// markdown decisions block because the parser uses an empty text to detect
// the end of a decision and would silently lose the entry.
func TestAddDecisionDropsEmptyText(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty string", ""},
		{"spaces only", "   "},
		{"tab and newline", "\t\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{}
			task.AddDecision(tc.text, "rationale", DecisionCategoryDesign)
			if len(task.Decisions) != 0 {
				t.Errorf("expected 0 decisions for empty text %q, got %d", tc.text, len(task.Decisions))
			}
		})
	}
}
