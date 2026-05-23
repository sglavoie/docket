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
