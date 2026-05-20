package workflow

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestApplyAction_AddTagSilentValidationFailure is a regression test for a bug
// where ActionAddTag returned true (claiming the task was modified) even when
// task.AddTag silently rejected the value due to validation (empty string,
// excess length, or forbidden characters such as commas/semicolons/newlines).
// applyAction must only report success when a tag was actually added.
func TestApplyAction_AddTagSilentValidationFailure(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"empty tag", ""},
		{"whitespace-only tag", "   "},
		{"comma in tag", "foo,bar"},
		{"semicolon in tag", "foo;bar"},
		{"newline in tag", "foo\nbar"},
		{"too-long tag", strings.Repeat("x", models.MaxTagLength+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "t1", Status: models.StatusTodo}
			action := RuleAction{ActionType: ActionAddTag, Value: tc.value}

			modified := applyAction(task, action)

			if modified {
				t.Errorf("expected modified=false for invalid tag %q (rejected by validation), got true", tc.value)
			}
			if len(task.Tags) != 0 {
				t.Errorf("expected zero tags for invalid value %q, got %v", tc.value, task.Tags)
			}
		})
	}

	t.Run("valid tag still reports modification", func(t *testing.T) {
		task := &models.Task{ID: "t1", Status: models.StatusTodo}
		action := RuleAction{ActionType: ActionAddTag, Value: "valid-tag"}

		if !applyAction(task, action) {
			t.Fatal("expected modified=true for a valid tag")
		}
		if !task.HasTag("valid-tag") {
			t.Errorf("expected tag to be present, got %v", task.Tags)
		}
	})
}
