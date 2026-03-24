package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestActivityRoundtripWithSeparatorInValues(t *testing.T) {
	// Bug: titles (or other values) containing " > " were mangled on roundtrip
	// because the parser splits on the first " > " to separate old/new values.
	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "title",
				OldValue:  "Use A > B pattern",
				NewValue:  "Use C > D pattern",
			},
		},
	}

	// Write activity block
	output := WriteActivity(task)

	// Parse it back
	parsedTask := &models.Task{
		Description: output,
	}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}

	entry := parsedTask.Activity[0]
	if entry.Field != "title" {
		t.Errorf("field = %q, want %q", entry.Field, "title")
	}
	if entry.OldValue != "Use A > B pattern" {
		t.Errorf("OldValue = %q, want %q", entry.OldValue, "Use A > B pattern")
	}
	if entry.NewValue != "Use C > D pattern" {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, "Use C > D pattern")
	}
}

func TestActivityRoundtripWithBackslashInValues(t *testing.T) {
	// Ensure backslashes in values survive roundtrip
	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "title",
				OldValue:  `path\to\file`,
				NewValue:  `new\path`,
			},
		},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}

	entry := parsedTask.Activity[0]
	if entry.OldValue != `path\to\file` {
		t.Errorf("OldValue = %q, want %q", entry.OldValue, `path\to\file`)
	}
	if entry.NewValue != `new\path` {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, `new\path`)
	}
}

func TestActivityRoundtripSingleValue(t *testing.T) {
	// Single-value entries (no old value) with " > " in the value
	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "tags",
				NewValue:  "+foo > bar",
			},
		},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}

	entry := parsedTask.Activity[0]
	if entry.OldValue != "" {
		t.Errorf("OldValue = %q, want empty", entry.OldValue)
	}
	if entry.NewValue != "+foo > bar" {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, "+foo > bar")
	}
}

func TestActivityRoundtripCreated(t *testing.T) {
	// "created" entries should not be affected by escaping
	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "created",
			},
		},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}

	entry := parsedTask.Activity[0]
	if entry.Field != "created" {
		t.Errorf("field = %q, want %q", entry.Field, "created")
	}
}

func TestActivityRoundtripPlainValues(t *testing.T) {
	// Values without " > " should work as before
	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "status",
				OldValue:  "todo",
				NewValue:  "in_progress",
			},
		},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}

	entry := parsedTask.Activity[0]
	if entry.OldValue != "todo" {
		t.Errorf("OldValue = %q, want %q", entry.OldValue, "todo")
	}
	if entry.NewValue != "in_progress" {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, "in_progress")
	}
}

func TestActivityBackwardsCompatibility(t *testing.T) {
	// Existing unescaped entries (from before the fix) should still parse.
	// This tests that the parser handles both escaped and unescaped " > ".
	// An unescaped entry with no " > " in values parses identically.
	task := &models.Task{
		Description: "<!-- ACTIVITY -->\n" +
			"<!-- act: 2025-06-15 10:30, status: todo > done -->\n" +
			"<!-- /ACTIVITY -->\n",
	}
	ExtractActivityFromDescription(task)

	if len(task.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(task.Activity))
	}

	entry := task.Activity[0]
	if entry.Field != "status" {
		t.Errorf("field = %q, want %q", entry.Field, "status")
	}
	if entry.OldValue != "todo" {
		t.Errorf("OldValue = %q, want %q", entry.OldValue, "todo")
	}
	if entry.NewValue != "done" {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, "done")
	}
}

func TestFindUnescapedSeparator(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"todo > done", 4},
		{`todo \> done`, -1},           // escaped, no unescaped separator
		{`A \> B > C \> D`, 6},         // first unescaped at index 6
		{"no separator here", -1},      // no separator
		{" > ", 0},                     // at start
		{`\> `, -1},                    // escaped at start
		{`hello \> world > final`, 14}, // first unescaped at index 14
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := findUnescapedSeparator(tt.input)
			if got != tt.want {
				t.Errorf("findUnescapedSeparator(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeUnescapeActivityValue(t *testing.T) {
	tests := []string{
		"simple value",
		"value with > arrow",
		"value with ` > ` separator",
		`value with \ backslash`,
		`value with \> fake escape`,
		"Use A > B > C",
		"",
	}

	for _, original := range tests {
		escaped := escapeActivityValue(original)
		unescaped := unescapeActivityValue(escaped)
		if unescaped != original {
			t.Errorf("roundtrip failed for %q: escaped=%q, unescaped=%q", original, escaped, unescaped)
		}
	}
}
