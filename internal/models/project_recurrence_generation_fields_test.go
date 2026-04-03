package models

import (
	"testing"
	"time"
)

func TestGenerateRecurringInstancesCopiesCorrectFields(t *testing.T) {
	parentID := "parent-1"
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Recurring Task",
		Description: "Weekly standup agenda",
		Status:      StatusDone,
		Priority:    2,
		Section:     SectionNext,
		Tags:        []string{"test"},
		DueDate:     &dueDate,
		Recurrence:  &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		ParentID:    &parentID,
		DependsOn:   []string{"dep-1", "dep-2"},
		Stage:       "Review",
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
	if newTask.Stage != "Review" {
		t.Errorf("Stage should be copied, got %q", newTask.Stage)
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
