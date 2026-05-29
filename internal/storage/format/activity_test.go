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

func TestWriteActivity_SkipsNilEntries(t *testing.T) {
	task := &models.Task{
		Activity: []*models.ActivityEntry{
			nil,
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "status",
				OldValue:  "todo",
				NewValue:  "done",
			},
		},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}
	if parsedTask.Activity[0].Field != "status" {
		t.Errorf("field = %q, want %q", parsedTask.Activity[0].Field, "status")
	}
}

func TestWriteActivity_AllNilEntriesWritesNothing(t *testing.T) {
	task := &models.Task{
		Activity: []*models.ActivityEntry{nil, nil},
	}

	if output := WriteActivity(task); output != "" {
		t.Fatalf("WriteActivity() = %q, want empty string", output)
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

func TestActivityRoundtripWithTrailingBackslash(t *testing.T) {
	// Bug: a value ending in backslash (e.g., a file path) was mis-parsed
	// because the escaped backslash (\\) before " > " made findUnescapedSeparator
	// think the separator was escaped when it was actually a literal backslash.
	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "title",
				OldValue:  `ends with\`,
				NewValue:  "new value",
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
	if entry.Field != "title" {
		t.Errorf("field = %q, want %q", entry.Field, "title")
	}
	if entry.OldValue != `ends with\` {
		t.Errorf("OldValue = %q, want %q", entry.OldValue, `ends with\`)
	}
	if entry.NewValue != "new value" {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, "new value")
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
		{`foo\\ > bar`, 5},             // double backslash (escaped literal \) before real separator
		{`foo\\\\ > bar`, 7},           // four backslashes (two escaped literals) before real separator
		{`foo\\\ > bar`, -1},           // three backslashes: two literal + one escaping the separator (odd=escaped)  //nolint:gocritic
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

func TestActivityRoundtripPreservesChronologicalOrder(t *testing.T) {
	// WriteActivity writes newest-first for human readability.
	// ExtractActivityFromDescription must reverse them back to chronological
	// order (oldest-first) so that diffAndRecordActivity and PruneActivity
	// work correctly (they append new entries at the end and keep the last N).
	ts1 := time.Date(2025, 1, 10, 8, 0, 0, 0, time.Local)
	ts2 := time.Date(2025, 1, 11, 9, 0, 0, 0, time.Local)
	ts3 := time.Date(2025, 1, 12, 10, 0, 0, 0, time.Local)

	task := &models.Task{
		ID:      "test-1",
		ShortID: 1,
		Type:    models.TaskTypeTask,
		Title:   "Test",
		Activity: []*models.ActivityEntry{
			{Timestamp: ts1, Field: "created"},
			{Timestamp: ts2, Field: "status", OldValue: "todo", NewValue: "in_progress"},
			{Timestamp: ts3, Field: "priority", OldValue: "3", NewValue: "1"},
		},
	}

	// Write and parse back
	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 3 {
		t.Fatalf("expected 3 activity entries, got %d", len(parsedTask.Activity))
	}

	// Verify chronological order is preserved (oldest first)
	if parsedTask.Activity[0].Field != "created" {
		t.Errorf("entry[0].Field = %q, want %q", parsedTask.Activity[0].Field, "created")
	}
	if parsedTask.Activity[1].Field != "status" {
		t.Errorf("entry[1].Field = %q, want %q", parsedTask.Activity[1].Field, "status")
	}
	if parsedTask.Activity[2].Field != "priority" {
		t.Errorf("entry[2].Field = %q, want %q", parsedTask.Activity[2].Field, "priority")
	}

	// Verify timestamps are in ascending order
	for i := 1; i < len(parsedTask.Activity); i++ {
		if parsedTask.Activity[i].Timestamp.Before(parsedTask.Activity[i-1].Timestamp) {
			t.Errorf("activity entries not in chronological order: entry[%d] (%v) before entry[%d] (%v)",
				i, parsedTask.Activity[i].Timestamp, i-1, parsedTask.Activity[i-1].Timestamp)
		}
	}
}

func TestActivityRoundtripEmptyNewValue(t *testing.T) {
	// Bug: clearing a field (e.g., stage: "Design" -> "") wrote
	// "stage: Design >  -->" which the regex trimmed back to "stage: Design >".
	// On reparse it became a single-value entry with NewValue="Design >",
	// destroying both the OldValue and the empty-NewValue semantic.
	cases := []struct {
		name  string
		field string
		old   string
		new   string
	}{
		{"stage cleared", "stage", "Design", ""},
		{"source cleared", "source", "github", ""},
		{"url cleared", "url", "https://example.com", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				Activity: []*models.ActivityEntry{{
					Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
					Field:     tc.field,
					OldValue:  tc.old,
					NewValue:  tc.new,
				}},
			}

			output := WriteActivity(task)
			parsedTask := &models.Task{Description: output}
			ExtractActivityFromDescription(parsedTask)

			if len(parsedTask.Activity) != 1 {
				t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
			}
			entry := parsedTask.Activity[0]
			if entry.Field != tc.field {
				t.Errorf("field = %q, want %q", entry.Field, tc.field)
			}
			if entry.OldValue != tc.old {
				t.Errorf("OldValue = %q, want %q", entry.OldValue, tc.old)
			}
			if entry.NewValue != tc.new {
				t.Errorf("NewValue = %q, want %q", entry.NewValue, tc.new)
			}
		})
	}
}

func TestActivityRoundtripLiteralEmptyMarkerValue(t *testing.T) {
	// Ensure a real value of "\e" (the sentinel) is not confused with empty.
	task := &models.Task{
		Activity: []*models.ActivityEntry{{
			Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
			Field:     "stage",
			OldValue:  "Design",
			NewValue:  `\e`,
		}},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(parsedTask.Activity))
	}
	entry := parsedTask.Activity[0]
	if entry.NewValue != `\e` {
		t.Errorf("NewValue = %q, want %q", entry.NewValue, `\e`)
	}
	if entry.OldValue != "Design" {
		t.Errorf("OldValue = %q, want %q", entry.OldValue, "Design")
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
		`trailing backslash\`,
		`double trailing\\`,
		`backslash before sep\ > rest`,
	}

	for _, original := range tests {
		escaped := escapeActivityValue(original)
		unescaped := unescapeActivityValue(escaped)
		if unescaped != original {
			t.Errorf("roundtrip failed for %q: escaped=%q, unescaped=%q", original, escaped, unescaped)
		}
	}
}

func TestActivityRoundtripWithFixedPointPayloads(t *testing.T) {
	task := &models.Task{
		Activity: []*models.ActivityEntry{
			{
				Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local),
				Field:     "title",
				OldValue:  "Old > title, status: todo",
				NewValue:  "New > title, key: value, ## Task: pseudo <!-- keep -->",
			},
			{
				Timestamp: time.Date(2025, 6, 15, 11, 0, 0, 0, time.Local),
				Field:     "source",
				OldValue:  "github",
				NewValue:  "cli, id: task-fixed",
			},
		},
	}

	output := WriteActivity(task)
	parsedTask := &models.Task{Description: output}
	ExtractActivityFromDescription(parsedTask)

	if len(parsedTask.Activity) != len(task.Activity) {
		t.Fatalf("expected %d activity entries, got %d", len(task.Activity), len(parsedTask.Activity))
	}

	for i, want := range task.Activity {
		got := parsedTask.Activity[i]
		if got.Field != want.Field {
			t.Errorf("activity[%d].Field = %q, want %q", i, got.Field, want.Field)
		}
		if !got.Timestamp.Equal(want.Timestamp) {
			t.Errorf("activity[%d].Timestamp = %v, want %v", i, got.Timestamp, want.Timestamp)
		}
		if got.OldValue != want.OldValue {
			t.Errorf("activity[%d].OldValue = %q, want %q", i, got.OldValue, want.OldValue)
		}
		if got.NewValue != want.NewValue {
			t.Errorf("activity[%d].NewValue = %q, want %q", i, got.NewValue, want.NewValue)
		}
	}

	if rewritten := WriteActivity(parsedTask); rewritten != output {
		t.Fatalf("activity block did not reach fixed point:\n--- output ---\n%s\n--- rewritten ---\n%s", output, rewritten)
	}
}
