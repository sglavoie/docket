package models

import (
	"testing"
	"time"
)

func TestTaskPromote(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	originalUpdate := task.UpdatedAt
	time.Sleep(time.Millisecond)

	err = task.Promote()
	if err != nil {
		t.Errorf("Promote() returned error: %v", err)
	}
	if task.Type != TaskTypeStory {
		t.Errorf("after Promote(), Type = %s, want story", task.Type)
	}
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after Promote()")
	}

	err = task.Promote()
	if err != nil {
		t.Errorf("second Promote() returned error: %v", err)
	}
	if task.Type != TaskTypeEpic {
		t.Errorf("after second Promote(), Type = %s, want epic", task.Type)
	}

	err = task.Promote()
	if err == nil {
		t.Error("Promote() on epic should return error")
	}
}

func TestTaskDemote(t *testing.T) {
	epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	originalUpdate := epic.UpdatedAt
	time.Sleep(time.Millisecond)

	err = epic.Demote()
	if err != nil {
		t.Errorf("Demote() returned error: %v", err)
	}
	if epic.Type != TaskTypeStory {
		t.Errorf("after Demote(), Type = %s, want story", epic.Type)
	}
	if !epic.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after Demote()")
	}

	err = epic.Demote()
	if err != nil {
		t.Errorf("second Demote() returned error: %v", err)
	}
	if epic.Type != TaskTypeTask {
		t.Errorf("after second Demote(), Type = %s, want task", epic.Type)
	}

	err = epic.Demote()
	if err == nil {
		t.Error("Demote() on task should return error")
	}
}

func TestTaskCycleLeafType(t *testing.T) {
	task, err := NewTask("task", 1, TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	originalUpdate := task.UpdatedAt
	time.Sleep(time.Millisecond)

	err = task.CycleLeafType(false)
	if err != nil {
		t.Errorf("CycleLeafType(false) returned error: %v", err)
	}
	if task.Type != TaskTypeBug {
		t.Errorf("after CycleLeafType(false), Type = %s, want bug", task.Type)
	}
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after CycleLeafType()")
	}

	err = task.CycleLeafType(false)
	if err != nil {
		t.Errorf("CycleLeafType(false) returned error: %v", err)
	}
	if task.Type != TaskTypeIdea {
		t.Errorf("after second CycleLeafType(false), Type = %s, want idea", task.Type)
	}

	err = task.CycleLeafType(true)
	if err != nil {
		t.Errorf("CycleLeafType(true) returned error: %v", err)
	}
	if task.Type != TaskTypeBug {
		t.Errorf("after CycleLeafType(true), Type = %s, want bug", task.Type)
	}
}
