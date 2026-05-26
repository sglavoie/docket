package cmd

import (
	"errors"
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

	originalClose := closeLogging
	defer func() { closeLogging = originalClose }()

	closed := false
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
