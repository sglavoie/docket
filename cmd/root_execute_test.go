package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Execute boundary tests: verify that Execute() returns errors instead of
// calling os.Exit, so main.go owns all process-lifecycle decisions.

// TestExecuteReturnsNilOnSuccess verifies Execute() returns nil when the
// command succeeds.  --version requires no config file and no TUI, so it
// exercises the happy path without side-effects.
func TestExecuteReturnsNilOnSuccess(t *testing.T) {
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := Execute(); err != nil {
		t.Fatalf("Execute() with --version = %v, want nil", err)
	}
}

// TestExecuteReturnsErrorNotExit verifies Execute() surfaces cobra errors as
// return values.  If Execute() still called os.Exit the test process would
// terminate and the failure would never be recorded.
func TestExecuteReturnsErrorNotExit(t *testing.T) {
	// Use a fresh command so we don't disturb global rootCmd state.
	failing := &cobra.Command{
		Use: "tt",
		RunE: func(*cobra.Command, []string) error {
			return errors.New("injected failure")
		},
	}

	err := executeCommand(failing)
	if err == nil {
		t.Fatal("Execute boundary: expected error, got nil")
	}
}

func TestExecuteCommandReturnsRunErrorAndClosesLogging(t *testing.T) {
	expected := errors.New("boom")
	command := &cobra.Command{
		Use: "tt",
		RunE: func(*cobra.Command, []string) error {
			return expected
		},
	}

	originalOpen, originalClose := openLogging, closeLogging
	defer func() { openLogging, closeLogging = originalOpen, originalClose }()

	opened, closed := false, false
	openLogging = func() {
		opened = true
	}
	closeLogging = func() {
		closed = true
	}

	err := executeCommand(command)
	if !errors.Is(err, expected) {
		t.Fatalf("executeCommand() error = %v, want %v", err, expected)
	}
	if !command.SilenceErrors {
		t.Fatal("executeCommand should silence cobra's default error printing")
	}
	if opened {
		t.Fatal("executeCommand should leave opening logging to the root pre-run hook")
	}
	if !closed {
		t.Fatal("executeCommand should close logging before returning")
	}
}

func TestExecuteCommandReturnsCobraValidationError(t *testing.T) {
	command := &cobra.Command{
		Use:  "tt",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			t.Fatal("RunE should not execute when args validation fails")
			return nil
		},
	}
	command.SetArgs([]string{"unexpected"})

	err := executeCommand(command)
	if err == nil {
		t.Fatal("executeCommand should return cobra's validation error")
	}
	if got := err.Error(); got == "" {
		t.Fatal("executeCommand returned an empty error")
	}
}

func TestRemovedCommandsFailAsUnknown(t *testing.T) {
	root := rootCmd
	previousSilenceErrors := root.SilenceErrors
	previousSilenceUsage := root.SilenceUsage
	root.SilenceErrors = true
	root.SilenceUsage = true
	t.Cleanup(func() {
		root.SetArgs(nil)
		root.SilenceErrors = previousSilenceErrors
		root.SilenceUsage = previousSilenceUsage
	})

	for _, path := range []string{
		"task",
		"note",
		"context",
		"search",
		"focus",
		"theme",
		"undo",
		"redo",
		"project new",
		"project delete",
		"conflict delete",
		"workspace delete",
	} {
		t.Run(strings.ReplaceAll(path, " ", "_"), func(t *testing.T) {
			root.SetArgs(strings.Fields(path))
			err := root.Execute()
			if err == nil {
				t.Fatalf("removed command %q is still accepted", path)
			}
			if !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("removed command %q error = %v, want unknown command", path, err)
			}
		})
	}
}

// TestStrayArgumentsAreRefusedByName keeps a mistyped subcommand suggesting
// the real one, a stray word on a leaf command naming that command, and every
// such refusal short: no usage dump, one pointer to --help.
func TestStrayArgumentsAreRefusedByName(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"doctr"}, []string{`unknown command "doctr" for "tt"`, "Did you mean this?", "\tdoctor", "Run 'tt --help' for usage"}},
		{[]string{"conflict", "lis"}, []string{`unknown command "lis" for "tt conflict"`, "\tlist", "Run 'tt conflict --help' for usage"}},
		{[]string{"doctor", "extra"}, []string{`tt doctor takes no arguments; got "extra"`, "Run 'tt doctor --help' for usage"}},
		{[]string{"conflict", "list", "extra"}, []string{`tt conflict list takes no arguments; got "extra"`}},
		{[]string{"workspace", "location", "extra"}, []string{`tt workspace location takes no arguments; got "extra"`}},
		{[]string{"conflict", "show"}, []string{"tt conflict show requires a conflict ID"}},
		{[]string{"doctor", "--bogus"}, []string{"unknown flag: --bogus", "Run 'tt doctor --help' for usage"}},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			var out strings.Builder
			rootCmd.SetArgs(tc.args)
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			t.Cleanup(func() {
				rootCmd.SetArgs(nil)
				rootCmd.SetOut(nil)
				rootCmd.SetErr(nil)
			})

			err := executeCommand(rootCmd)
			if err == nil {
				t.Fatalf("tt %s was accepted", strings.Join(tc.args, " "))
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
			if strings.Contains(out.String(), "Usage:") {
				t.Errorf("refusal printed the usage block:\n%s", out.String())
			}
		})
	}
}

// TestNoArgLeafCommandsCompleteNoFiles keeps Tab from offering file names to a
// command that takes no arguments.
func TestNoArgLeafCommandsCompleteNoFiles(t *testing.T) {
	for _, command := range []*cobra.Command{doctorCmd, conflictListCmd, workspaceLocationCmd} {
		if command.ValidArgsFunction == nil {
			t.Errorf("%s has no ValidArgsFunction", command.CommandPath())
			continue
		}
		if _, directive := command.ValidArgsFunction(command, nil, ""); directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("%s completion directive = %v, want NoFileComp", command.CommandPath(), directive)
		}
	}
}
