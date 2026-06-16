package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestDiffTaskDueTimeOfDayChange exercises the activity diff when only the
// clock component of a time-bearing due date changes (same calendar day).
// Regression: the previous formatter rendered both values as YYYY-MM-DD, so
// the diff silently dropped the change and no activity entry was recorded.
func TestDiffTaskDueTimeOfDayChange(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	morning := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)
	old.DueDate = &morning
	old.HasTime = true

	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	evening := time.Date(2024, 1, 15, 18, 30, 0, 0, time.Local)
	newTask.DueDate = &evening
	newTask.HasTime = true

	diffTask(old, newTask)

	found := findActivity(newTask, "due")
	if found == nil {
		t.Fatal("expected due activity entry for time-of-day change")
	}
	if found.OldValue == found.NewValue {
		t.Errorf("expected distinct old/new values, both were %q", found.OldValue)
	}
}

// TestDiffTaskDueDateOnlyUnchanged ensures the date-only path still treats
// equal days as unchanged (does not regress the common case).
func TestDiffTaskDueDateOnlyUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	d := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
	old.DueDate = &d
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	d2 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
	newTask.DueDate = &d2

	diffTask(old, newTask)

	if findActivity(newTask, "due") != nil {
		t.Fatal("expected no due activity entry when date unchanged")
	}
}

func TestDiffTaskRecurrenceAdded(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	diffTask(old, newTask)

	found := findActivity(newTask, "recurrence")
	if found == nil {
		t.Fatal("expected recurrence activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "daily" {
		t.Errorf("expected none→daily, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskRecurrenceRemoved(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	diffTask(old, newTask)

	found := findActivity(newTask, "recurrence")
	if found == nil {
		t.Fatal("expected recurrence activity entry")
	}
	if found.OldValue != "weekly" || found.NewValue != "none" {
		t.Errorf("expected weekly→none, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskRecurrenceUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	diffTask(old, newTask)

	if findActivity(newTask, "recurrence") != nil {
		t.Fatal("expected no recurrence activity entry when unchanged")
	}
}

func TestDiffTaskChecklistAdded(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Checklist = []models.ChecklistItem{
		{Text: "item 1", Done: false},
		{Text: "item 2", Done: true},
	}

	diffTask(old, newTask)

	found := findActivity(newTask, "checklist")
	if found == nil {
		t.Fatal("expected checklist activity entry")
	}
	if found.OldValue != "0/0" || found.NewValue != "1/2" {
		t.Errorf("expected 0/0→1/2, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskChecklistItemCompleted(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Checklist = []models.ChecklistItem{
		{Text: "item 1", Done: false},
		{Text: "item 2", Done: false},
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Checklist = []models.ChecklistItem{
		{Text: "item 1", Done: true},
		{Text: "item 2", Done: false},
	}

	diffTask(old, newTask)

	found := findActivity(newTask, "checklist")
	if found == nil {
		t.Fatal("expected checklist activity entry")
	}
	if found.OldValue != "0/2" || found.NewValue != "1/2" {
		t.Errorf("expected 0/2→1/2, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskChecklistUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Checklist = []models.ChecklistItem{{Text: "item", Done: true}}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Checklist = []models.ChecklistItem{{Text: "item", Done: true}}

	diffTask(old, newTask)

	if findActivity(newTask, "checklist") != nil {
		t.Fatal("expected no checklist activity entry when unchanged")
	}
}

func TestDiffTaskScoreDimensions(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.ScoreDimensions = map[string]int{"impact": 5}

	diffTask(old, newTask)

	found := findActivity(newTask, "score")
	if found == nil {
		t.Fatal("expected score activity entry")
	}
}

func TestDiffTaskScoreDimensionsUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.ScoreDimensions = map[string]int{"impact": 5}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.ScoreDimensions = map[string]int{"impact": 5}

	diffTask(old, newTask)

	if findActivity(newTask, "score") != nil {
		t.Fatal("expected no score activity entry when unchanged")
	}
}

func TestDiffTaskDimmed(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Dimmed = true

	diffTask(old, newTask)

	found := findActivity(newTask, "dimmed")
	if found == nil {
		t.Fatal("expected dimmed activity entry")
	}
	if found.OldValue != "false" || found.NewValue != "true" {
		t.Errorf("expected false→true, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskDimmedUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Dimmed = true
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Dimmed = true

	diffTask(old, newTask)

	if findActivity(newTask, "dimmed") != nil {
		t.Fatal("expected no dimmed activity entry when unchanged")
	}
}

func TestDiffTaskURL(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.URL = "https://example.com"

	diffTask(old, newTask)

	found := findActivity(newTask, "url")
	if found == nil {
		t.Fatal("expected url activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "https://example.com" {
		t.Errorf("expected none→https://example.com, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskSource(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Source = "github"

	diffTask(old, newTask)

	found := findActivity(newTask, "source")
	if found == nil {
		t.Fatal("expected source activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "github" {
		t.Errorf("expected none→github, got %s→%s", found.OldValue, found.NewValue)
	}
}
