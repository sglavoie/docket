package models

import (
	"testing"
	"time"
)

func TestIsSnoozed_Nil(t *testing.T) {
	task := &Task{}
	if task.IsSnoozed() {
		t.Error("Task with nil SnoozedUntil should not be snoozed")
	}
}

func TestIsSnoozed_Future(t *testing.T) {
	future := time.Now().AddDate(0, 0, 7)
	task := &Task{SnoozedUntil: &future}
	if !task.IsSnoozed() {
		t.Error("Task with future SnoozedUntil should be snoozed")
	}
}

func TestIsSnoozed_Today(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	task := &Task{SnoozedUntil: &today}
	if task.IsSnoozed() {
		t.Error("Task snoozed until today should NOT be snoozed (it should reappear today)")
	}
}

func TestIsSnoozed_Past(t *testing.T) {
	past := time.Now().AddDate(0, 0, -3)
	task := &Task{SnoozedUntil: &past}
	if task.IsSnoozed() {
		t.Error("Task with past SnoozedUntil should not be snoozed")
	}
}

func TestClearSnooze(t *testing.T) {
	future := time.Now().AddDate(0, 0, 7)
	task := &Task{SnoozedUntil: &future}
	task.ClearSnooze()
	if task.SnoozedUntil != nil {
		t.Error("ClearSnooze should set SnoozedUntil to nil")
	}
}
