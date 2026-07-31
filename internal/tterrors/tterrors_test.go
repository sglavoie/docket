package tterrors

import (
	"errors"
	"testing"
)

func TestLiveErrorConstructorsWrapTheirSentinels(t *testing.T) {
	if err := NewProjectNotFoundError("p1"); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("project error = %v", err)
	}
	inner := errors.New("disk full")
	err := NewSaveProjectError("Alpha", inner)
	if !errors.Is(err, ErrSaveFailed) || !errors.Is(err, inner) {
		t.Fatalf("save error = %v", err)
	}
}
