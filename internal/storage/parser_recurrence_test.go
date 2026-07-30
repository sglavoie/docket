package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdownWithCanonicalRecurrence(t *testing.T) {
	content := `# Project: Test Project

## Task: Daily
<!-- id: task1, state: ready, priority: 2, recur: 1d -->

## Task: Biweekly
<!-- id: task2, state: ready, priority: 2, recur: 2w -->

## Task: Quarterly
<!-- id: task3, state: ready, priority: 2, recur: 3m -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("ParseTasksMarkdown: %v", err)
	}
	want := []models.RecurrencePattern{
		{Interval: 1, Unit: models.RecurrenceDaily},
		{Interval: 2, Unit: models.RecurrenceWeekly},
		{Interval: 3, Unit: models.RecurrenceMonthly},
	}
	if len(tasks) != len(want) {
		t.Fatalf("tasks = %d, want %d", len(tasks), len(want))
	}
	for i := range want {
		if tasks[i].Recurrence == nil || tasks[i].Recurrence.Interval != want[i].Interval || tasks[i].Recurrence.Unit != want[i].Unit {
			t.Errorf("task %d recurrence = %+v, want %+v", i, tasks[i].Recurrence, want[i])
		}
	}
}

func TestParseTasksMarkdownDropsUnsupportedLegacyRecurrence(t *testing.T) {
	content := `# Project: Test Project

## Task: Legacy
<!-- id: task1, state: ready, priority: 2, recur: 1w:mon,wed,fri -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("ParseTasksMarkdown: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Recurrence != nil {
		t.Fatalf("legacy recurrence survived live parsing: %+v", tasks)
	}
}

func TestCanonicalRecurrenceRoundTrip(t *testing.T) {
	for _, recurrence := range []*models.RecurrencePattern{
		{Interval: 1, Unit: models.RecurrenceDaily},
		{Interval: 3, Unit: models.RecurrenceDaily},
		{Interval: 1, Unit: models.RecurrenceWeekly},
		{Interval: 2, Unit: models.RecurrenceWeekly},
		{Interval: 1, Unit: models.RecurrenceMonthly},
		{Interval: 3, Unit: models.RecurrenceMonthly},
	} {
		t.Run(recurrence.String(), func(t *testing.T) {
			due := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
			task, err := models.NewGenericTask("task1", 1, "Recurring Task")
			if err != nil {
				t.Fatal(err)
			}
			task.DueDate = &due
			task.Recurrence = recurrence

			content := WriteTasksMarkdown("Test", []*models.Task{task})
			_, parsed, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("ParseTasksMarkdown: %v", err)
			}
			if len(parsed) != 1 || parsed[0].Recurrence == nil ||
				parsed[0].Recurrence.Interval != recurrence.Interval ||
				parsed[0].Recurrence.Unit != recurrence.Unit {
				t.Fatalf("round-trip recurrence = %+v, want %+v", parsed, recurrence)
			}
		})
	}
}

func TestWriteTasksMarkdownWithCanonicalRecurrence(t *testing.T) {
	task, err := models.NewGenericTask("task1", 1, "Biweekly Task")
	if err != nil {
		t.Fatal(err)
	}
	task.Recurrence = &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}

	content := WriteTasksMarkdown("Test", []*models.Task{task})
	if !strings.Contains(content, "recur: 2w") {
		t.Fatalf("canonical recurrence missing from Markdown:\n%s", content)
	}
}
