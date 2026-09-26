package tterrors

import (
	"errors"
	"testing"
)

func TestLiveErrorConstructorsWrapTheirSentinels(t *testing.T) {
	if err := NewProjectNotFoundError("p1"); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("project error = %v", err)
	} else if got, want := err.Error(), `project not found: "p1"`; got != want {
		t.Fatalf("project error text = %q, want %q", got, want)
	}
	inner := errors.New("disk full")
	err := NewSaveProjectError("Alpha", inner)
	if !errors.Is(err, ErrSaveFailed) || !errors.Is(err, inner) {
		t.Fatalf("save error = %v", err)
	}
}
