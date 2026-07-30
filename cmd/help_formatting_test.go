package cmd

import (
	"slices"
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

func TestPublicCommandTreeIsMaintenanceOnly(t *testing.T) {
	root := RootCmd()
	root.InitDefaultHelpCmd()

	var paths []string
	visit(root, func(c *cobra.Command) {
		paths = append(paths, c.CommandPath())
		if len(c.Aliases) != 0 {
			t.Errorf("%s retains aliases %v", c.CommandPath(), c.Aliases)
		}
	})
	slices.Sort(paths)

	want := []string{
		"tt",
		"tt completion",
		"tt conflict",
		"tt conflict list",
		"tt conflict resolve",
		"tt conflict show",
		"tt doctor",
		"tt help",
		"tt project",
		"tt project export",
		"tt workspace",
		"tt workspace location",
		"tt workspace move",
	}
	slices.Sort(want)
	if !slices.Equal(paths, want) {
		t.Fatalf("public command paths = %v, want %v", paths, want)
	}

	for _, retired := range []string{"config", "verbose"} {
		if flag := root.PersistentFlags().Lookup(retired); flag != nil {
			t.Errorf("retired global flag --%s remains registered", retired)
		}
	}
	for _, retained := range []string{"data-dir", "no-color"} {
		if flag := root.PersistentFlags().Lookup(retained); flag == nil {
			t.Errorf("retained global flag --%s is missing", retained)
		}
	}
}
