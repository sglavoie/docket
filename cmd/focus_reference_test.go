package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestCLIReferenceFocusUsesOutlinerTerms locks the `tt focus queue` help (the
// source of truth for cmd/cli-reference.md) to the state-driven Outliner model.
func TestCLIReferenceFocusUsesOutlinerTerms(t *testing.T) {
	queueHelp := focusQueueCmd.Short + "\n" + focusQueueCmd.Long
	todayHelp := focusTodayCmd.Short + "\n" + focusTodayCmd.Long

	// Corrected help must teach the state-driven Next model and shipped keys.
	for _, term := range []string{"Outliner", "Next", "StateNext", "ordering-only", "Tab", "'q'", "'w'"} {
		if !strings.Contains(queueHelp, term) {
			t.Errorf("focus queue help missing required Outliner term %q:\n%s", term, queueHelp)
		}
	}
	for _, term := range []string{"built-in Today section", "All mode", "'w'"} {
		if !strings.Contains(todayHelp, term) {
			t.Errorf("focus today help missing required Outliner term %q:\n%s", term, todayHelp)
		}
	}

	for _, term := range []string{"planner", "cockpit", "focus view", "focus board", "destination switcher", "ctrl+g", ";q", ";w"} {
		if strings.Contains(strings.ToLower(queueHelp+"\n"+todayHelp), term) {
			t.Errorf("focus help references retired TUI guidance %q:\n%s\n%s", term, queueHelp, todayHelp)
		}
	}

	// The generated reference on disk must have been regenerated from the
	// corrected command help (`just docs`).
	data, err := os.ReadFile("cli-reference.md")
	if err != nil {
		t.Fatalf("read cli-reference.md: %v", err)
	}
	ref := string(data)
	for _, term := range []string{"Outliner's Next", "built-in Today section", "ordering-only"} {
		if !strings.Contains(ref, term) {
			t.Errorf("cli-reference.md missing corrected focus term %q; run `just docs`", term)
		}
	}
}

// TestCLIHelpContainsNoRetiredTUIGuidance scans the live Cobra tree rather than
// a generated snapshot so newly added command or flag help cannot reintroduce
// navigation for TUI surfaces that no longer ship. The phrases are intentionally
// exact: real CLI concepts such as `tt dash`, dashboard summaries,
// `tt project list`, and `tt focus queue` remain valid.
func TestCLIHelpContainsNoRetiredTUIGuidance(t *testing.T) {
	retired := []string{
		"planner",
		"cockpit",
		"focus board",
		"focus view",
		"destination switcher",
		"ctrl+g",
		";q",
		";w",
	}

	check := func(owner, text string) {
		t.Helper()
		lower := strings.ToLower(text)
		for _, term := range retired {
			if strings.Contains(lower, term) {
				t.Errorf("%s contains retired TUI guidance %q: %q", owner, term, text)
			}
		}
	}

	visit(RootCmd(), func(command *cobra.Command) {
		path := command.CommandPath()
		check(path+" Short", command.Short)
		check(path+" Long", command.Long)
		check(path+" Example", command.Example)

		checkFlags := func(scope string, flags *pflag.FlagSet) {
			flags.VisitAll(func(flag *pflag.Flag) {
				check(path+" "+scope+" --"+flag.Name, flag.Usage)
			})
		}
		checkFlags("flag", command.LocalFlags())
		checkFlags("persistent flag", command.PersistentFlags())
	})
}
