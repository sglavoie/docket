package models

import (
	"strings"
	"testing"
)

func TestNormalizeTaskContextUsesTagCanonicalizationWithContextErrors(t *testing.T) {
	got, err := NormalizeTaskContext("  #Platform API  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "platform-api" {
		t.Fatalf("normalized context = %q, want %q", got, "platform-api")
	}

	for _, raw := range []string{"", "!!!", strings.Repeat("a", MaxTagLength+1)} {
		_, err := NormalizeTaskContext(raw)
		if err == nil {
			t.Fatalf("NormalizeTaskContext(%q) succeeded", raw)
		}
		if strings.Contains(err.Error(), "tag") || !strings.Contains(err.Error(), "context") {
			t.Fatalf("NormalizeTaskContext(%q) error = %q, want context vocabulary", raw, err)
		}
	}
}

func TestValidateTaskContextAcceptsUnsetAndRejectsNonCanonical(t *testing.T) {
	if err := ValidateTaskContext(""); err != nil {
		t.Fatalf("unset context rejected: %v", err)
	}
	if err := ValidateTaskContext("platform-api"); err != nil {
		t.Fatalf("canonical context rejected: %v", err)
	}
	err := ValidateTaskContext("Platform API")
	if err == nil || strings.Contains(err.Error(), "tag") || !strings.Contains(err.Error(), "context") {
		t.Fatalf("validation error = %v, want context vocabulary", err)
	}
}

func TestTaskContextMutationsNormalizeAndStampOnlyOnChange(t *testing.T) {
	task, err := NewGenericTask("task-1", 1, "Context methods")
	if err != nil {
		t.Fatal(err)
	}
	initial := task.UpdatedAt

	task.SetContext("  Platform API ")
	if task.Context != "platform-api" || !task.HasContext("#platform api") {
		t.Fatalf("context = %q, HasContext = %v", task.Context, task.HasContext("#platform api"))
	}
	if !task.UpdatedAt.After(initial) {
		t.Fatalf("SetContext did not advance UpdatedAt: %v", task.UpdatedAt)
	}

	setAt := task.UpdatedAt
	task.SetContext("platform-api")
	if !task.UpdatedAt.Equal(setAt) {
		t.Fatal("setting the same context changed UpdatedAt")
	}
	task.SetContext("!!!")
	if task.Context != "platform-api" || !task.UpdatedAt.Equal(setAt) {
		t.Fatal("invalid context changed the task")
	}

	task.ClearContext()
	if task.Context != "" || !task.UpdatedAt.After(setAt) {
		t.Fatalf("ClearContext left context %q or did not stamp UpdatedAt", task.Context)
	}
	clearedAt := task.UpdatedAt
	task.ClearContext()
	if !task.UpdatedAt.Equal(clearedAt) {
		t.Fatal("clearing an unset context changed UpdatedAt")
	}
}

func TestHasTagDoesNotMatchContext(t *testing.T) {
	task := &Task{Tags: []string{"bug"}, Context: "platform"}
	if task.HasTag("platform") {
		t.Fatal("HasTag matched the task context")
	}
	if !task.HasTag("bug") {
		t.Fatal("HasTag stopped matching ordinary tags")
	}
}
