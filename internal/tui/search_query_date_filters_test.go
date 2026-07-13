package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesIsFilter_OverdueUsesStartOfDay(t *testing.T) {
	// A task due today at midnight should NOT be overdue during that day.
	// The is:overdue filter must use day-level comparison (StartOfDay),
	// matching Task.IsOverdue() and matchesDueDateFilter "overdue".
	today := time.Now()
	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	task := &models.Task{
		WorkflowState: models.StateReady,
		DueDate:       &todayMidnight,
	}

	filters := &ParsedSearchQuery{IsState: []string{"overdue"}}
	if TaskMatchesFilters(task, filters, nil) {
		t.Error("task due today should NOT match is:overdue")
	}

	// A task due yesterday should be overdue.
	yesterday := todayMidnight.AddDate(0, 0, -1)
	task2 := &models.Task{
		WorkflowState: models.StateReady,
		DueDate:       &yesterday,
	}
	if !TaskMatchesFilters(task2, filters, nil) {
		t.Error("task due yesterday should match is:overdue")
	}

	// Completed tasks should never be overdue.
	task3 := &models.Task{
		WorkflowState: models.StateDone,
		DueDate:       &yesterday,
	}
	if TaskMatchesFilters(task3, filters, nil) {
		t.Error("completed task should NOT match is:overdue")
	}
}

func TestMatchesIsFilter_SnoozedUsesStartOfDay(t *testing.T) {
	// A task snoozed until today should NOT be considered snoozed (it becomes
	// visible today). The is:snoozed filter must use day-level comparison
	// matching Task.IsSnoozed().
	today := time.Now()
	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	task := &models.Task{
		SnoozedUntil: &todayMidnight,
	}

	filters := &ParsedSearchQuery{IsState: []string{"snoozed"}}
	if TaskMatchesFilters(task, filters, nil) {
		t.Error("task snoozed until today should NOT match is:snoozed")
	}

	// A task snoozed until tomorrow should be snoozed.
	tomorrow := todayMidnight.AddDate(0, 0, 1)
	task2 := &models.Task{
		SnoozedUntil: &tomorrow,
	}
	if !TaskMatchesFilters(task2, filters, nil) {
		t.Error("task snoozed until tomorrow should match is:snoozed")
	}

	// A task with no snooze should not be snoozed.
	task3 := &models.Task{}
	if TaskMatchesFilters(task3, filters, nil) {
		t.Error("task with no snooze should NOT match is:snoozed")
	}
}

func TestIsOverdueConsistentWithDueDateFilter(t *testing.T) {
	// Verify is:overdue and @overdue agree on all edge cases.
	today := time.Now()
	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	yesterday := todayMidnight.AddDate(0, 0, -1)
	tomorrow := todayMidnight.AddDate(0, 0, 1)

	tests := []struct {
		name string
		task *models.Task
	}{
		{"due today", &models.Task{WorkflowState: models.StateReady, DueDate: &todayMidnight}},
		{"due yesterday", &models.Task{WorkflowState: models.StateReady, DueDate: &yesterday}},
		{"due tomorrow", &models.Task{WorkflowState: models.StateReady, DueDate: &tomorrow}},
		{"no due date", &models.Task{WorkflowState: models.StateReady}},
		{"completed + overdue", &models.Task{WorkflowState: models.StateDone, DueDate: &yesterday}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isFilter := &ParsedSearchQuery{IsState: []string{"overdue"}}
			dueDateFilter := &ParsedSearchQuery{DueDateFilter: "overdue"}

			isResult := TaskMatchesFilters(tt.task, isFilter, nil)
			dueDateResult := TaskMatchesFilters(tt.task, dueDateFilter, nil)

			if isResult != dueDateResult {
				t.Errorf("is:overdue=%v but @overdue=%v — these should agree", isResult, dueDateResult)
			}
		})
	}
}
