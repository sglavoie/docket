package models

import (
	"testing"
	"time"
)

func TestGenerateRecurringInstancesCopiesCorrectFields(t *testing.T) {
	parentID := "parent-1"
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task := &Task{
		ID:            "task-1",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Recurring Task",
		Description:   "Weekly standup agenda",
		WorkflowState: StateDone,
		Priority:      2,
		Tags:          []string{"test"},
		DueDate:       &dueDate,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		ParentID:      &parentID,
		DependsOn:     []string{"dep-1", "dep-2"},
		Checklist: []ChecklistItem{
			{Text: "Item 1", Done: true},
			{Text: "Item 2", Done: false},
		},
		ScoreDimensions: map[string]int{"impact": 4, "effort": 2},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]

	if newTask.Title != "Recurring Task" {
		t.Errorf("Title should be copied, got %q", newTask.Title)
	}
	if newTask.Priority != 2 {
		t.Errorf("Priority should be copied, got %d", newTask.Priority)
	}
	if len(newTask.Tags) != 1 || newTask.Tags[0] != "test" {
		t.Errorf("Tags should be copied, got %v", newTask.Tags)
	}
	if newTask.Description != "Weekly standup agenda" {
		t.Errorf("Description should be copied, got %q", newTask.Description)
	}
	// The legacy Stage is intentionally NOT carried forward: stages are
	// retired, and copying a review-stage would derive the new instance into
	// the review state instead of the inbox it is created for.
	if newTask.State() != StateInbox {
		t.Errorf("new instance should be in the inbox state, got %s", newTask.State())
	}
	if len(newTask.ScoreDimensions) != 2 || newTask.ScoreDimensions["impact"] != 4 || newTask.ScoreDimensions["effort"] != 2 {
		t.Errorf("ScoreDimensions should be copied, got %v", newTask.ScoreDimensions)
	}

	if len(newTask.Checklist) != 2 {
		t.Errorf("Checklist should be copied, got %d items", len(newTask.Checklist))
	} else {
		if newTask.Checklist[0].Text != "Item 1" || newTask.Checklist[0].Done {
			t.Errorf("Checklist item 0 should be unchecked copy, got %+v", newTask.Checklist[0])
		}
		if newTask.Checklist[1].Text != "Item 2" || newTask.Checklist[1].Done {
			t.Errorf("Checklist item 1 should be unchecked copy, got %+v", newTask.Checklist[1])
		}
	}

	if newTask.ParentID != nil {
		t.Errorf("ParentID should not be copied, got %v", newTask.ParentID)
	}
	if len(newTask.DependsOn) != 0 {
		t.Errorf("DependsOn should not be copied, got %v", newTask.DependsOn)
	}
}

func TestGenerateRecurringInstancesCopiesEstimatedDaysAndStartDate(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	startDate := time.Date(2024, 1, 10, 9, 0, 0, 0, time.UTC)

	task := &Task{
		ID:            "task-1",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Estimated work",
		WorkflowState: StateDone,
		Priority:      2,
		DueDate:       &dueDate,
		StartDate:     &startDate,
		EstimatedDays: 3.5,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.EstimatedDays != 3.5 {
		t.Errorf("EstimatedDays should be carried forward, got %v", newTask.EstimatedDays)
	}
	if newTask.StartDate == nil {
		t.Fatal("StartDate should be carried forward, got nil")
	}
	if !newTask.StartDate.Equal(startDate) {
		t.Errorf("StartDate should equal template start date, got %v", newTask.StartDate)
	}
	// Ensure deep copy — mutating the new task's StartDate must not affect the template.
	newStart := newTask.StartDate
	if newStart == task.StartDate {
		t.Error("StartDate pointer should be a copy, not shared with template")
	}
}

func TestGenerateRecurringInstancesNilStartDateStaysNil(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task := &Task{
		ID:            "task-1",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "No start date",
		WorkflowState: StateDone,
		Priority:      2,
		DueDate:       &dueDate,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.StartDate != nil {
		t.Errorf("StartDate should remain nil, got %v", newTask.StartDate)
	}
	if newTask.EstimatedDays != 0 {
		t.Errorf("EstimatedDays should remain zero, got %v", newTask.EstimatedDays)
	}
}

func TestGenerateRecurringInstancesCopiesHasTime(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 14, 30, 0, 0, time.Local)

	task := &Task{
		ID:            "task-1",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Daily standup",
		WorkflowState: StateDone,
		Priority:      3,
		DueDate:       &dueDate,
		HasTime:       true,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if !newTask.HasTime {
		t.Error("HasTime should be carried forward to the new recurring instance")
	}
	if newTask.DueDate == nil {
		t.Fatal("new task should have a due date")
	}
}

func TestGenerateRecurringInstancesNoHasTimeWhenFalse(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)

	task := &Task{
		ID:            "task-2",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Weekly report",
		WorkflowState: StateDone,
		Priority:      3,
		DueDate:       &dueDate,
		HasTime:       false,
		Recurrence:    &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]
	if newTask.HasTime {
		t.Error("HasTime should be false when source task has HasTime=false")
	}
}

func TestGenerateRecurringInstances_NewInstanceProperties(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC)
	task := &Task{
		ID:            "props-task",
		ShortID:       1,
		Type:          TaskTypeBug,
		Title:         "Fix login issue",
		WorkflowState: StateDone,
		Priority:      3,
		Tags:          []string{"backend", "auth", "urgent"},
		DueDate:       &dueDate,
		CompletedAt:   &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:  2,
			Unit:      RecurrenceWeekly,
			Weekdays:  []int{1, 3, 5},
			Skipped:   4,
			Streak:    2,
			MaxStreak: 7,
		},
	}

	project := &Project{
		Name:        "Props Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
		Sections:    []string{SectionInbox, SectionNext},
	}

	project.GenerateRecurringInstances()

	if len(project.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	origTask := project.Tasks[1]

	if newTask.State() != StateInbox {
		t.Errorf("new task state = %s, want %s", newTask.State(), StateInbox)
	}
	if newTask.Type != TaskTypeBug {
		t.Errorf("new task type = %s, want %s", newTask.Type, TaskTypeBug)
	}
	if newTask.Title != "Fix login issue" {
		t.Errorf("new task title = %q, want %q", newTask.Title, "Fix login issue")
	}
	if newTask.Priority != 3 {
		t.Errorf("new task priority = %d, want 3", newTask.Priority)
	}
	if len(newTask.Tags) != 3 {
		t.Fatalf("new task tags length = %d, want 3", len(newTask.Tags))
	}
	if newTask.Tags[0] != "backend" || newTask.Tags[1] != "auth" || newTask.Tags[2] != "urgent" {
		t.Errorf("new task tags = %v, want [backend auth urgent]", newTask.Tags)
	}
	newTask.Tags[0] = "mutated"
	if len(origTask.Tags) > 0 && origTask.Tags[0] == "mutated" {
		t.Error("tag slice should be a deep copy, not shared")
	}

	if newTask.Recurrence == nil {
		t.Fatal("new task recurrence should not be nil")
	}
	if newTask.Recurrence.Interval != 2 {
		t.Errorf("recurrence interval = %d, want 2", newTask.Recurrence.Interval)
	}
	if newTask.Recurrence.Unit != RecurrenceWeekly {
		t.Errorf("recurrence unit = %s, want %s", newTask.Recurrence.Unit, RecurrenceWeekly)
	}
	if len(newTask.Recurrence.Weekdays) != 3 || newTask.Recurrence.Weekdays[0] != 1 {
		t.Errorf("recurrence weekdays = %v, want [1 3 5]", newTask.Recurrence.Weekdays)
	}
	newTask.Recurrence.Weekdays[0] = 99

	if newTask.Recurrence.Skipped != 4 {
		t.Errorf("recurrence skipped = %d, want 4", newTask.Recurrence.Skipped)
	}
	if newTask.Recurrence.Streak != 3 {
		t.Errorf("recurrence streak = %d, want 3", newTask.Recurrence.Streak)
	}
	if newTask.Recurrence.MaxStreak != 7 {
		t.Errorf("recurrence max streak = %d, want 7 (unchanged)", newTask.Recurrence.MaxStreak)
	}

	if newTask.DueDate == nil {
		t.Error("new task due date should not be nil")
	}
	if newTask.ID == origTask.ID {
		t.Error("new task should have a different ID from original")
	}
	if newTask.ShortID != 2 {
		t.Errorf("new task short ID = %d, want 2", newTask.ShortID)
	}
	if newTask.CreatedAt.IsZero() {
		t.Error("new task CreatedAt should be set")
	}
	if newTask.UpdatedAt.IsZero() {
		t.Error("new task UpdatedAt should be set")
	}

	if origTask.Recurrence != nil {
		t.Error("original task recurrence should be nil after generation")
	}
}

func TestGenerateRecurringInstances_OriginalDayCarriedForward(t *testing.T) {
	dueDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	completedAt := time.Date(2025, 1, 31, 10, 0, 0, 0, time.UTC)
	task := &Task{
		ID:            "monthly-31",
		ShortID:       1,
		Type:          TaskTypeTask,
		Title:         "Monthly on 31st",
		WorkflowState: StateDone,
		DueDate:       &dueDate,
		CompletedAt:   &completedAt,
		Recurrence: &RecurrencePattern{
			Interval:    1,
			Unit:        RecurrenceMonthly,
			OriginalDay: 31,
		},
	}

	project := &Project{
		Name:        "OriginalDay Test",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count := project.GenerateRecurringInstances()
	if count != 1 {
		t.Fatalf("expected 1 instance generated, got %d", count)
	}

	newTask := project.Tasks[0]
	if newTask.Recurrence == nil {
		t.Fatal("new task should have recurrence pattern")
	}
	if newTask.Recurrence.OriginalDay != 31 {
		t.Errorf("OriginalDay = %d, want 31 (must be carried forward to prevent month drift)", newTask.Recurrence.OriginalDay)
	}
}
