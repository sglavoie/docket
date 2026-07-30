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

func TestIsDueThisWeek(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// No due date
	if task.IsDueThisWeek() {
		t.Error("expected not due this week when no due date")
	}

	now := time.Now()

	// Due today — should be this week
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	task.DueDate = &today
	if !task.IsDueThisWeek() {
		t.Error("expected due this week for today")
	}

	// Due in 3 days — should be this week
	in3Days := now.AddDate(0, 0, 3)
	task.DueDate = &in3Days
	if !task.IsDueThisWeek() {
		t.Error("expected due this week for +3 days")
	}

	// Due in 6 days — should still be this week
	in6Days := now.AddDate(0, 0, 6)
	task.DueDate = &in6Days
	if !task.IsDueThisWeek() {
		t.Error("expected due this week for +6 days")
	}

	// Due in 7 days — should NOT be this week (exclusive)
	in7Days := now.AddDate(0, 0, 7)
	task.DueDate = &in7Days
	if task.IsDueThisWeek() {
		t.Error("expected not due this week for exactly +7 days")
	}

	// Due yesterday — should NOT be this week
	yesterday := now.AddDate(0, 0, -1)
	task.DueDate = &yesterday
	if task.IsDueThisWeek() {
		t.Error("expected not due this week for yesterday")
	}
}
