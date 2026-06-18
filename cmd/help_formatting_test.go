package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestNoTabsInLongHelp ensures that the Long help text of every command uses
// consistent space indentation. Stray tab characters in Long strings render as
// misaligned blocks in terminal help output (and in the embedded
// cli-reference.md document).
func TestNoTabsInLongHelp(t *testing.T) {
	root := RootCmd()
	var offenders []string
	visit(root, func(c *cobra.Command) {
		if strings.Contains(c.Long, "\t") {
			offenders = append(offenders, c.CommandPath())
		}
	})
	if len(offenders) > 0 {
		t.Fatalf("commands with tab characters in Long help: %v", offenders)
	}
}

func visit(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		visit(sub, fn)
	}
}

// TestScoreSubcmdsHaveArgsConstraint verifies that score sub-commands whose
// Use string declares required positional arguments (<>) also declare a Cobra
// Args constraint. Without the constraint, Cobra accepts zero arguments and
// the command fails with a hand-rolled error instead of the standard usage
// message.
func TestScoreSubcmdsHaveArgsConstraint(t *testing.T) {
	root := RootCmd()
	cases := []struct {
		path string
	}{
		{"tt score show"},
		{"tt score set"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			var found *cobra.Command
			visit(root, func(c *cobra.Command) {
				if c.CommandPath() == tc.path {
					found = c
				}
			})
			if found == nil {
				t.Fatalf("command %q not found", tc.path)
			}
			// A command has a non-nil Args validator only if it was explicitly set.
			// Calling found.Args directly fails because it requires os.Args — instead
			// we check the exported ValidArgs or use the unexported sentinel via
			// string comparison of the UseLine, but the reliable path is to verify
			// that RunE refuses a zero-arg call gracefully (Cobra enforces Args
			// before RunE, so a nil Args means Cobra passes through).
			//
			// We test this indirectly: execute the command with no args and confirm
			// Cobra returns an error (due to Args enforcement) before RunE runs.
			// Because RunE would try to load config (I/O), we can't run it in tests.
			// Instead, check that Args field is non-nil by running ValidateArgs.
			if err := found.ValidateArgs([]string{}); err == nil {
				t.Errorf("command %q accepts 0 args with no error; expected Args constraint to reject empty call", tc.path)
			}
		})
	}
}

// TestTaskMoveToUseLine verifies that "task move-to" advertises optional args
// in its Use string, matching its RangeArgs(0, 2) declaration. The Use string
// must use bracket notation ([task-id] [dest-project]) so that generated help
// and the embedded cli-reference.md reflect the interactive-selection fallback.
func TestTaskMoveToUseLine(t *testing.T) {
	root := RootCmd()
	var moveToCmd *cobra.Command
	visit(root, func(c *cobra.Command) {
		if c.CommandPath() == "tt task move-to" {
			moveToCmd = c
		}
	})
	if moveToCmd == nil {
		t.Fatal("command 'tt task move-to' not found")
	}
	use := moveToCmd.Use
	if strings.Contains(use, "<task-id>") || strings.Contains(use, "<dest-project>") {
		t.Errorf("Use line %q uses required-arg <> syntax but the command accepts 0-2 args; use [] brackets instead", use)
	}
}
