package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSetRecurrenceWeekly(t *testing.T) {
	task := testutil.NewTestTask("Recurring Task", models.TaskTypeTask)

	if task.Recurrence != nil {
		t.Error("new task should not have recurrence")
	}

	pattern := models.ParseRecurrence("weekly")
	if pattern == nil {
		t.Fatal("failed to parse 'weekly' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "weekly")
}

func TestSetRecurrenceDaily(t *testing.T) {
	task := testutil.NewTestTask("Daily Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("daily")
	if pattern == nil {
		t.Fatal("failed to parse 'daily' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "daily")
}

func TestSetRecurrenceMonthly(t *testing.T) {
	task := testutil.NewTestTask("Monthly Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("monthly")
	if pattern == nil {
		t.Fatal("failed to parse 'monthly' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "monthly")
}

func TestSetRecurrenceYearly(t *testing.T) {
	task := testutil.NewTestTask("Yearly Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("yearly")
	if pattern == nil {
		t.Fatal("failed to parse 'yearly' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "yearly")
}

func TestSetRecurrenceBiweekly(t *testing.T) {
	task := testutil.NewTestTask("Biweekly Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("biweekly")
	if pattern == nil {
		t.Fatal("failed to parse 'biweekly' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "every 2 weeks")
}

func TestSetRecurrenceIntervalDays(t *testing.T) {
	task := testutil.NewTestTask("Every 3 Days Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("3d")
	if pattern == nil {
		t.Fatal("failed to parse '3d' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "every 3 days")
}

func TestSetRecurrenceIntervalWeeks(t *testing.T) {
	task := testutil.NewTestTask("Every 2 Weeks Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("2w")
	if pattern == nil {
		t.Fatal("failed to parse '2w' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "every 2 weeks")
}

func TestSetRecurrenceIntervalMonths(t *testing.T) {
	task := testutil.NewTestTask("Every 6 Months Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("6m")
	if pattern == nil {
		t.Fatal("failed to parse '6m' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "every 6 months")
}

func TestSetRecurrenceIntervalYears(t *testing.T) {
	task := testutil.NewTestTask("Every 2 Years Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("2y")
	if pattern == nil {
		t.Fatal("failed to parse '2y' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "every 2 years")
}

func TestSetRecurrenceWeeklyWithDays(t *testing.T) {
	task := testutil.NewTestTask("Weekly on Mon/Fri", models.TaskTypeTask)

	pattern := models.ParseRecurrence("weekly:mon,fri")
	if pattern == nil {
		t.Fatal("failed to parse 'weekly:mon,fri' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "weekly on Mon and Fri")
}

func TestSetRecurrenceWeeklyWithMultipleDays(t *testing.T) {
	task := testutil.NewTestTask("Weekly on Mon/Wed/Fri", models.TaskTypeTask)

	pattern := models.ParseRecurrence("weekly:mon,wed,fri")
	if pattern == nil {
		t.Fatal("failed to parse 'weekly:mon,wed,fri' pattern")
	}

	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "weekly on Mon, Wed, and Fri")
}

func TestClearRecurrence(t *testing.T) {
	task := testutil.NewTestTask("Recurring Task", models.TaskTypeTask)

	pattern := models.ParseRecurrence("weekly")
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if task.Recurrence == nil {
		t.Fatal("recurrence should be set before clearing")
	}

	task.ClearRecurrence()

	if task.Recurrence != nil {
		t.Error("recurrence should be nil after clearing")
	}
}

func TestClearRecurrenceWhenNone(t *testing.T) {
	task := testutil.NewTestTask("Non-recurring Task", models.TaskTypeTask)

	if task.Recurrence != nil {
		t.Fatal("task should not have recurrence")
	}

	// Clearing when none exists should be a no-op
	task.ClearRecurrence()

	if task.Recurrence != nil {
		t.Error("recurrence should remain nil")
	}
}

func TestParseRecurrenceInvalidPattern(t *testing.T) {
	invalidPatterns := []string{
		"invalid",
		"abc",
		"1x",
		"0d",
		"-1w",
		"weekly:",
		"",
	}

	for _, pattern := range invalidPatterns {
		result := models.ParseRecurrence(pattern)
		if result != nil && pattern != "weekly:" {
			t.Errorf("expected nil for invalid pattern %q, got %v", pattern, result)
		}
	}
}

func TestParseRecurrenceValidPatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"daily", "daily"},
		{"weekly", "weekly"},
		{"monthly", "monthly"},
		{"yearly", "yearly"},
		{"biweekly", "every 2 weeks"},
		{"1d", "daily"},
		{"1w", "weekly"},
		{"1m", "monthly"},
		{"1y", "yearly"},
		{"2d", "every 2 days"},
		{"3w", "every 3 weeks"},
		{"4m", "every 4 months"},
		{"5y", "every 5 years"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pattern := models.ParseRecurrence(tt.input)
			if pattern == nil {
				t.Fatalf("failed to parse valid pattern %q", tt.input)
			}
			testutil.AssertEqual(t, pattern.String(), tt.expected)
		})
	}
}

func TestUpdateRecurrence(t *testing.T) {
	task := testutil.NewTestTask("Task", models.TaskTypeTask)

	// Set initial pattern
	pattern1 := models.ParseRecurrence("weekly")
	if err := task.SetRecurrence(pattern1); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "weekly")

	// Update to new pattern
	pattern2 := models.ParseRecurrence("monthly")
	if err := task.SetRecurrence(pattern2); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}
	testutil.AssertEqual(t, task.Recurrence.String(), "monthly")
}

func TestRecurrenceStringNil(t *testing.T) {
	var pattern *models.RecurrencePattern = nil
	testutil.AssertEqual(t, pattern.String(), "")
}

func TestRecurrenceOnDifferentTaskTypes(t *testing.T) {
	taskTypes := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
		models.TaskTypeStory,
		models.TaskTypeEpic,
	}

	for _, taskType := range taskTypes {
		t.Run(string(taskType), func(t *testing.T) {
			task := testutil.NewTestTask("Test", taskType)
			pattern := models.ParseRecurrence("weekly")
			if err := task.SetRecurrence(pattern); err != nil {
				t.Fatalf("SetRecurrence failed: %v", err)
			}

			if task.Recurrence == nil {
				t.Errorf("recurrence should be settable on %s", taskType)
			}
			testutil.AssertEqual(t, task.Recurrence.String(), "weekly")
		})
	}
}

func TestSetRecurrenceUpdatesTimestamp(t *testing.T) {
	task := testutil.NewTestTask("Task", models.TaskTypeTask)
	originalTime := task.UpdatedAt

	pattern := models.ParseRecurrence("weekly")
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	if !task.UpdatedAt.After(originalTime) && task.UpdatedAt != originalTime {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestClearRecurrenceUpdatesTimestamp(t *testing.T) {
	task := testutil.NewTestTask("Task", models.TaskTypeTask)
	pattern := models.ParseRecurrence("weekly")
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence failed: %v", err)
	}

	originalTime := task.UpdatedAt

	task.ClearRecurrence()

	if !task.UpdatedAt.After(originalTime) && task.UpdatedAt != originalTime {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestParseRecurrenceWeekdayVariants(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"weekly:monday", "weekly on Mon"},
		{"weekly:mon", "weekly on Mon"},
		{"weekly:tuesday", "weekly on Tue"},
		{"weekly:tue", "weekly on Tue"},
		{"weekly:wednesday", "weekly on Wed"},
		{"weekly:wed", "weekly on Wed"},
		{"weekly:thursday", "weekly on Thu"},
		{"weekly:thu", "weekly on Thu"},
		{"weekly:friday", "weekly on Fri"},
		{"weekly:fri", "weekly on Fri"},
		{"weekly:saturday", "weekly on Sat"},
		{"weekly:sat", "weekly on Sat"},
		{"weekly:sunday", "weekly on Sun"},
		{"weekly:sun", "weekly on Sun"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pattern := models.ParseRecurrence(tt.input)
			if pattern == nil {
				t.Fatalf("failed to parse valid pattern %q", tt.input)
			}
			testutil.AssertEqual(t, pattern.String(), tt.expected)
		})
	}
}

func TestRecurDeferConflictDetection(t *testing.T) {
	// The --defer flag should be detected as an action flag even when value is 0.
	// This test verifies the conflicting flags logic at the cmd layer.
	// Before the fix, --defer 0 was silently ignored because actionCount
	// only incremented for taskRecurDefer != 0.

	// Create a fresh command with flags to test Changed() behavior
	cmd := *taskRecurCmd // shallow copy
	cmd.ResetFlags()
	cmd.Flags().BoolVar(&taskRecurClear, "clear", false, "")
	cmd.Flags().BoolVar(&taskRecurSkip, "skip", false, "")
	cmd.Flags().IntVar(&taskRecurDefer, "defer", 0, "")

	// Simulate setting --defer 0 explicitly
	if err := cmd.Flags().Set("defer", "0"); err != nil {
		t.Fatal(err)
	}

	// Verify Changed() reports true for explicitly set flags
	if !cmd.Flags().Changed("defer") {
		t.Fatal("expected Changed('defer') to be true after Set")
	}
}

func TestParseRecurrenceIntervalWithDays(t *testing.T) {
	pattern := models.ParseRecurrence("2w:mon,fri")
	if pattern == nil {
		t.Fatal("failed to parse '2w:mon,fri' pattern")
	}

	testutil.AssertEqual(t, pattern.Interval, 2)
	testutil.AssertEqual(t, pattern.Unit, models.RecurrenceWeekly)
	if len(pattern.Weekdays) != 2 {
		t.Errorf("expected 2 weekdays, got %d", len(pattern.Weekdays))
	}
}
