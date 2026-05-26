package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

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
