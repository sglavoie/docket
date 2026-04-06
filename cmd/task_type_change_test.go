package cmd

import "testing"

func TestUndoActionLabelsForTypeChange(t *testing.T) {
	undoActions := map[string]string{
		"promote": "Promote Task",
		"demote":  "Demote Task",
		"cycle":   "Cycle Task Type",
	}

	for _, action := range []string{"promote", "demote", "cycle"} {
		label, ok := undoActions[action]
		if !ok {
			t.Errorf("missing undo action label for %q", action)
			continue
		}
		if label == "" {
			t.Errorf("undo action label for %q is empty", action)
		}
	}

	if undoActions["demote"] == undoActions["promote"] {
		t.Error("demote undo label should differ from promote")
	}
	if undoActions["cycle"] == undoActions["promote"] {
		t.Error("cycle undo label should differ from promote")
	}
}
