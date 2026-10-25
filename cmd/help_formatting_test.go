package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestNoTabsInLongHelp ensures that the Long help text of every command uses
// consistent space indentation. Stray tab characters in Long strings render as
// misaligned blocks in terminal help output.
func TestNoTabsInLongHelp(t *testing.T) {
	root := rootCmd
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

// TestExamplesLiveInTheExampleField keeps examples where cobra renders them,
// under one "Examples:" heading, and holds every root example to a command
// line that runs.
func TestExamplesLiveInTheExampleField(t *testing.T) {
	visit(rootCmd, func(c *cobra.Command) {
		for _, heading := range []string{"Examples:", "EXAMPLES"} {
			if strings.Contains(c.Long, heading) {
				t.Errorf("%s keeps examples in Long help; move them to Example", c.CommandPath())
			}
		}
	})
	if strings.TrimSpace(rootCmd.Example) == "" {
		t.Errorf("%s has no Example", rootCmd.CommandPath())
	}
	// Every runnable leaf carries one, so no command's help ends without showing
	// how it is spelled.
	visit(rootCmd, func(c *cobra.Command) {
		if c.Hidden || c.Name() == "help" || !c.Runnable() || c.HasSubCommands() {
			return
		}
		if strings.TrimSpace(c.Example) == "" {
			t.Errorf("%s has no Example", c.CommandPath())
		}
	})
	for _, line := range strings.Split(rootCmd.Example, "\n") {
		if strings.TrimSpace(line) == "docket project export" {
			t.Errorf("root example %q fails without a slug or --all", line)
		}
	}
	if completionCmd.Use != "completion {bash|zsh|fish}" {
		t.Errorf("completion Use = %q, want its shell argument marked required", completionCmd.Use)
	}
}

func visit(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		visit(sub, fn)
	}
}

func TestPublicCommandTreeIsMaintenanceOnly(t *testing.T) {
	root := rootCmd
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
		"docket",
		"docket completion",
		"docket conflict",
		"docket conflict list",
		"docket conflict resolve",
		"docket conflict show",
		"docket doctor",
		"docket help",
		"docket project",
		"docket project export",
		"docket sync",
		"docket sync conflicts",
		"docket sync diagnostics",
		"docket sync doctor",
		"docket sync enroll",
		"docket sync exchange",
		"docket sync join",
		"docket sync resolve",
		"docket sync status",
		"docket workspace",
		"docket workspace backup",
		"docket workspace location",
		"docket workspace move",
		"docket workspace verify-backup",
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
