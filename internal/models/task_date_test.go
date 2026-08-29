package models

import (
	"testing"
	"time"
)

func TestIsDueToday(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// No due date
	if task.IsDueToday() {
		t.Error("expected not due today when no due date")
	}

	// Due today
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, time.Local)
	task.DueDate = &today
	if !task.IsDueToday() {
		t.Error("expected due today")
	}

	// Due tomorrow
	tomorrow := now.AddDate(0, 0, 1)
	task.DueDate = &tomorrow
	if task.IsDueToday() {
		t.Error("expected not due today when due tomorrow")
	}

	// Due yesterday
	yesterday := now.AddDate(0, 0, -1)
	task.DueDate = &yesterday
	if task.IsDueToday() {
		t.Error("expected not due today when due yesterday")
	}
}
