package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMetadata_RecurrenceStreakFields(t *testing.T) {
	task := &models.Task{}
	ParseMetadata(task, "id: abc, status: todo, priority: 1, recur: 1w, recur.skipped: 3, recur.streak: 5, recur.max_streak: 8")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", task.Recurrence.Skipped)
	}
	if task.Recurrence.Streak != 5 {
		t.Errorf("Streak = %d, want 5", task.Recurrence.Streak)
	}
	if task.Recurrence.MaxStreak != 8 {
		t.Errorf("MaxStreak = %d, want 8", task.Recurrence.MaxStreak)
	}
}

func TestParseMetadata_RecurrenceStreakFieldsIgnoredWithoutRecur(t *testing.T) {
	task := &models.Task{}
	warnings := ParseMetadata(task, "id: abc, status: todo, priority: 1, recur.streak: 5")

	// Without a recur field, Recurrence is nil so the streak fields are ignored
	if task.Recurrence != nil {
		t.Errorf("Recurrence should be nil when recur key is absent")
	}
	// Should produce an orphaned sub-fields warning
	found := false
	for _, w := range warnings {
		if w.Field == "recur" && strings.Contains(w.Message, "sub-fields") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about orphaned recurrence sub-fields")
	}
}

func TestWriteMetadata_RecurrenceStreakFields(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:  1,
			Unit:      models.RecurrenceWeekly,
			Skipped:   2,
			Streak:    4,
			MaxStreak: 7,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if !strings.Contains(result, "recur.skipped: 2") {
		t.Errorf("expected recur.skipped: 2, got: %s", result)
	}
	if !strings.Contains(result, "recur.streak: 4") {
		t.Errorf("expected recur.streak: 4, got: %s", result)
	}
	if !strings.Contains(result, "recur.max_streak: 7") {
		t.Errorf("expected recur.max_streak: 7, got: %s", result)
	}
}

func TestWriteMetadata_RecurrenceStreakFieldsOmittedWhenZero(t *testing.T) {
	task := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval: 1,
			Unit:     models.RecurrenceDaily,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	result := WriteMetadata(task)
	if strings.Contains(result, "recur.skipped") {
		t.Errorf("should not contain recur.skipped when 0, got: %s", result)
	}
	if strings.Contains(result, "recur.streak") {
		t.Errorf("should not contain recur.streak when 0, got: %s", result)
	}
	if strings.Contains(result, "recur.max_streak") {
		t.Errorf("should not contain recur.max_streak when 0, got: %s", result)
	}
}

func TestParseMetadata_RecurrenceSubFieldsBeforeRecurKey(t *testing.T) {
	task := &models.Task{}
	// Sub-fields appear BEFORE the recur key — should still be applied
	ParseMetadata(task, "id: abc, status: todo, priority: 1, recur.skipped: 2, recur.streak: 4, recur.max_streak: 9, recur: 1w")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", task.Recurrence.Skipped)
	}
	if task.Recurrence.Streak != 4 {
		t.Errorf("Streak = %d, want 4", task.Recurrence.Streak)
	}
	if task.Recurrence.MaxStreak != 9 {
		t.Errorf("MaxStreak = %d, want 9", task.Recurrence.MaxStreak)
	}
}

func TestMetadata_RecurrenceStreakRoundtrip(t *testing.T) {
	original := &models.Task{
		ID:       "abc",
		Status:   models.StatusTodo,
		Priority: 1,
		Recurrence: &models.RecurrencePattern{
			Interval:  2,
			Unit:      models.RecurrenceWeekly,
			Weekdays:  []int{1, 3, 5},
			Skipped:   3,
			Streak:    6,
			MaxStreak: 10,
		},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
	}

	meta := WriteMetadata(original)
	metaStr := strings.TrimPrefix(meta, "<!-- ")
	metaStr = strings.TrimSuffix(metaStr, " -->")

	parsed := &models.Task{}
	ParseMetadata(parsed, metaStr)

	if !recurrenceEqual(parsed.Recurrence, original.Recurrence) {
		t.Errorf("Roundtrip failed:\n  got:  %+v\n  want: %+v", parsed.Recurrence, original.Recurrence)
	}
}

// TestParseMetadata_RecurrenceWeekdayCommas verifies that comma-separated weekday
// lists inside a recur value (e.g., "1w:mon,wed,fri") are not mis-split by the
// metadata parser, which uses ", key:" as its delimiter rather than bare commas.
func TestParseMetadata_RecurrenceWeekdayCommas(t *testing.T) {
	task := &models.Task{}
	warnings := ParseMetadata(task, "id: abc, status: todo, priority: 1, recur: 1w:mon,wed,fri")

	if task.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if task.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("Unit = %v, want weekly", task.Recurrence.Unit)
	}
	if task.Recurrence.Interval != 1 {
		t.Errorf("Interval = %d, want 1", task.Recurrence.Interval)
	}
	// mon=1, wed=3, fri=5
	if len(task.Recurrence.Weekdays) != 3 {
		t.Errorf("Weekdays = %v, want [1 3 5] (mon,wed,fri)", task.Recurrence.Weekdays)
	}
	for _, w := range warnings {
		if w.Field == "recur" {
			t.Errorf("unexpected recurrence warning: %s", w.Message)
		}
	}
}
