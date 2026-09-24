package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestRunReturnsZeroWithoutWritingOnSuccess(t *testing.T) {
	var stderr bytes.Buffer

	exitCode := run(func() error {
		return nil
	}, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunReportsErrorAndReturnsFailure(t *testing.T) {
	var stderr bytes.Buffer
	errBoom := errors.New("boom")

	exitCode := run(func() error {
		return errBoom
	}, &stderr)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if got, want := stderr.String(), "boom\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunReturnsFailureWithNilStderr(t *testing.T) {
	errBoom := errors.New("boom")

	exitCode := run(func() error {
		return errBoom
	}, nil)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
}

type exitCodeError struct{ code int }

func (e exitCodeError) Error() string { return "" }
func (e exitCodeError) ExitCode() int { return e.code }

func TestRunHonorsAnErrorsOwnExitCode(t *testing.T) {
	var stderr bytes.Buffer

	exitCode := run(func() error {
		return fmt.Errorf("wrapped: %w", exitCodeError{code: 2})
	}, &stderr)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if got, want := stderr.String(), "wrapped: \n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunReturnsTwoAndStaysQuietForASilentExitCodeError(t *testing.T) {
	var stderr bytes.Buffer

	exitCode := run(func() error {
		return exitCodeError{code: 2}
	}, &stderr)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
