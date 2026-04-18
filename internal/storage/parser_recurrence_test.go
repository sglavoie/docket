package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTasksMarkdown_WithRecurrence(t *testing.T) {
	content := `# Project: Test Project

## Task: Daily Standup
<!-- id: task1, status: todo, priority: 2, recur: 1d -->

Daily standup meeting.

## Task: Weekly Review
<!-- id: task2, status: todo, priority: 2, recur: 1w -->

Weekly review.

## Task: MWF Meeting
<!-- id: task3, status: todo, priority: 2, recur: 1w:mon,wed,fri -->

Meeting on Mon, Wed, Fri.
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Daily task
	if tasks[0].Recurrence == nil {
		t.Error("task1 should have recurrence")
	} else {
		if tasks[0].Recurrence.Interval != 1 {
			t.Errorf("task1 recurrence interval = %d, want 1", tasks[0].Recurrence.Interval)
		}
		if tasks[0].Recurrence.Unit != models.RecurrenceDaily {
			t.Errorf("task1 recurrence unit = %s, want day", tasks[0].Recurrence.Unit)
		}
	}

	// Weekly task
	if tasks[1].Recurrence == nil {
		t.Error("task2 should have recurrence")
	} else {
		if tasks[1].Recurrence.Interval != 1 {
			t.Errorf("task2 recurrence interval = %d, want 1", tasks[1].Recurrence.Interval)
		}
		if tasks[1].Recurrence.Unit != models.RecurrenceWeekly {
			t.Errorf("task2 recurrence unit = %s, want week", tasks[1].Recurrence.Unit)
		}
	}

	// Weekly with weekdays
	if tasks[2].Recurrence == nil {
		t.Error("task3 should have recurrence")
	} else {
		if tasks[2].Recurrence.Interval != 1 {
			t.Errorf("task3 recurrence interval = %d, want 1", tasks[2].Recurrence.Interval)
		}
		if tasks[2].Recurrence.Unit != models.RecurrenceWeekly {
			t.Errorf("task3 recurrence unit = %s, want week", tasks[2].Recurrence.Unit)
		}
		expectedDays := []int{1, 3, 5}
		if len(tasks[2].Recurrence.Weekdays) != len(expectedDays) {
			t.Errorf("task3 weekdays = %v, want %v", tasks[2].Recurrence.Weekdays, expectedDays)
		} else {
			for i, d := range expectedDays {
				if tasks[2].Recurrence.Weekdays[i] != d {
					t.Errorf("task3 weekdays[%d] = %d, want %d", i, tasks[2].Recurrence.Weekdays[i], d)
				}
			}
		}
	}
}

func TestParseTasksMarkdown_RecurrenceFromCompletion(t *testing.T) {
	content := `# Project: Test Project

## Task: Daily From Completion
<!-- id: task1, status: todo, priority: 1, recur: 1d~ -->

## Task: Weekly MWF From Completion
<!-- id: task2, status: todo, priority: 1, recur: 2w:mon,fri~ -->
`
	_, tasks, err := ParseTasksMarkdown(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	t1 := tasks[0]
	if t1.Recurrence == nil {
		t.Fatal("task1: Recurrence is nil")
	}
	if t1.Recurrence.Mode != models.RecurModeFromCompletion {
		t.Errorf("task1: Mode = %q, want %q", t1.Recurrence.Mode, models.RecurModeFromCompletion)
	}
	if t1.Recurrence.Interval != 1 || t1.Recurrence.Unit != models.RecurrenceDaily {
		t.Errorf("task1: got interval=%d unit=%s, want 1 day", t1.Recurrence.Interval, t1.Recurrence.Unit)
	}

	t2 := tasks[1]
	if t2.Recurrence == nil {
		t.Fatal("task2: Recurrence is nil")
	}
	if t2.Recurrence.Mode != models.RecurModeFromCompletion {
		t.Errorf("task2: Mode = %q, want %q", t2.Recurrence.Mode, models.RecurModeFromCompletion)
	}
	if t2.Recurrence.Interval != 2 || t2.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("task2: got interval=%d unit=%s, want 2 week", t2.Recurrence.Interval, t2.Recurrence.Unit)
	}
	wantDays := []int{1, 5}
	if len(t2.Recurrence.Weekdays) != len(wantDays) {
		t.Errorf("task2: weekdays = %v, want %v", t2.Recurrence.Weekdays, wantDays)
	}
}

func TestRecurrenceRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		recurrence *models.RecurrencePattern
	}{
		{"daily", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
		{"every 3 days", &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceDaily}},
		{"weekly", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}},
		{"biweekly", &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly}},
		{"monthly", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly}},
		{"quarterly", &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceMonthly}},
		{"yearly", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceYearly}},
		{"every 2 years", &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceYearly}},
		{"weekly mon", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1}}},
		{"weekly mon,wed,fri", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 3, 5}}},
		{"biweekly tue,thu", &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly, Weekdays: []int{2, 4}}},
		{"weekly all days", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}}},
		// from-completion mode
		{"daily from-completion", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromCompletion}},
		{"every 3 days from-completion", &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromCompletion}},
		{"weekly:mon,fri from-completion", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 5}, Mode: models.RecurModeFromCompletion}},
		{"monthly from-completion", &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly, Mode: models.RecurModeFromCompletion}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
			original, err := models.NewTask("task1", 1, models.TaskTypeTask, "Recurring Task")
			if err != nil {
				t.Fatal(err)
			}
			original.DueDate = &dueDate
			original.Recurrence = tc.recurrence

			content := WriteTasksMarkdown("Test", []*models.Task{original})

			_, tasks, err := ParseTasksMarkdown(content)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			if len(tasks) != 1 {
				t.Fatalf("expected 1 task, got %d", len(tasks))
			}

			parsed := tasks[0]
			if parsed.Recurrence == nil {
				t.Fatal("parsed task should have recurrence")
			}

			if parsed.Recurrence.Interval != tc.recurrence.Interval {
				t.Errorf("interval mismatch: got %d, want %d", parsed.Recurrence.Interval, tc.recurrence.Interval)
			}
			if parsed.Recurrence.Unit != tc.recurrence.Unit {
				t.Errorf("unit mismatch: got %s, want %s", parsed.Recurrence.Unit, tc.recurrence.Unit)
			}
			if len(parsed.Recurrence.Weekdays) != len(tc.recurrence.Weekdays) {
				t.Errorf("weekdays length mismatch: got %v, want %v", parsed.Recurrence.Weekdays, tc.recurrence.Weekdays)
			} else {
				for i, wd := range tc.recurrence.Weekdays {
					if parsed.Recurrence.Weekdays[i] != wd {
						t.Errorf("weekday[%d] mismatch: got %d, want %d", i, parsed.Recurrence.Weekdays[i], wd)
					}
				}
			}
			if parsed.Recurrence.Mode != tc.recurrence.Mode {
				t.Errorf("mode mismatch: got %q, want %q", parsed.Recurrence.Mode, tc.recurrence.Mode)
			}
		})
	}
}

func TestWriteTasksMarkdown_WithRecurrence(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task1, err := models.NewTask("task1", 1, models.TaskTypeTask, "Daily Task")
	if err != nil {
		t.Fatal(err)
	}
	task1.DueDate = &dueDate
	task1.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	task2, err := models.NewTask("task2", 2, models.TaskTypeTask, "Weekly MWF Task")
	if err != nil {
		t.Fatal(err)
	}
	task2.DueDate = &dueDate
	task2.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 3, 5}}

	content := WriteTasksMarkdown("Test", []*models.Task{task1, task2})

	if !strings.Contains(content, "recur: 1d") {
		t.Error("expected markdown to contain 'recur: 1d'")
	}
	if !strings.Contains(content, "recur: 1w:mon,wed,fri") {
		t.Error("expected markdown to contain 'recur: 1w:mon,wed,fri'")
	}
}
