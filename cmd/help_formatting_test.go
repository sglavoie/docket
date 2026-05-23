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
