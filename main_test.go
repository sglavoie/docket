package main

import (
	"bytes"
	"errors"
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
