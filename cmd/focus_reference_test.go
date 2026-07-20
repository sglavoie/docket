package cmd

import (
	"os"
	"strings"
	"testing"
)

// TestCLIReferenceFocusUsesPlannerTerms locks the `tt focus queue` help (the
// source of truth for cmd/cli-reference.md) to the state-driven Planner model.
// It guards against retired-view language ("focus view") and obsolete
// state-independent-queue semantics silently returning to maintained help.
func TestCLIReferenceFocusUsesPlannerTerms(t *testing.T) {
	help := focusQueueCmd.Short + "\n" + focusQueueCmd.Long

	// Retired planning view must not resurface in the generated reference.
	for _, term := range []string{"focus view", "Focus View"} {
		if strings.Contains(help, term) {
			t.Errorf("focus queue help references retired view %q; describe the Planner instead:\n%s", term, help)
		}
	}

	// Corrected help must teach the state-driven Next model and Planner terms.
	for _, term := range []string{"Planner", "Next", "StateNext", "ordering-only"} {
		if !strings.Contains(help, term) {
			t.Errorf("focus queue help missing required planner term %q:\n%s", term, help)
		}
	}

	// ;q opens Planner/Next and ;w opens Planner/Today; Backlog promotion is
	// through q (Next) or w (Today).
	for _, want := range []string{"';q'", "';w'"} {
		if !strings.Contains(help, want) {
			t.Errorf("focus queue help should reference planner shortcut %q:\n%s", want, help)
		}
	}

	// The generated reference on disk must have been regenerated from the
	// corrected command help (`just docs`).
	data, err := os.ReadFile("cli-reference.md")
	if err != nil {
		t.Fatalf("read cli-reference.md: %v", err)
	}
	ref := string(data)
	if strings.Contains(ref, "TUI's focus view") {
		t.Errorf("cli-reference.md still references the retired focus view; run `just docs`")
	}
	if !strings.Contains(ref, "ordering-only") {
		t.Errorf("cli-reference.md not regenerated from corrected focus queue help; run `just docs`")
	}
}
