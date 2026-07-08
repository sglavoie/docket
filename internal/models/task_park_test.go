package models

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func TestTaskPark(t *testing.T) {
	task, err := NewTask("task-park", 1, TaskTypeTask, "Resume me")
	if err != nil {
		t.Fatal(err)
	}

	task.Park(" next step ")

	if !task.IsParked() {
		t.Fatal("task should be parked")
	}
	if task.ParkedNote != "next step" {
		t.Errorf("ParkedNote = %q, want %q", task.ParkedNote, "next step")
	}
	if task.ParkedAt == nil {
		t.Fatal("ParkedAt should be set")
	}
}

func TestTaskReparkRollsPreviousNoteToComment(t *testing.T) {
	task, err := NewTask("task-repark", 1, TaskTypeTask, "Resume me")
	if err != nil {
		t.Fatal(err)
	}
	parkedAt := time.Date(2026, 7, 8, 10, 15, 0, 0, time.Local)
	task.ParkedNote = "old state"
	task.ParkedAt = &parkedAt

	task.Park("new state")

	if task.ParkedNote != "new state" {
		t.Errorf("ParkedNote = %q, want %q", task.ParkedNote, "new state")
	}
	if len(task.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(task.Comments))
	}
	wantPrefix := "[parked " + parkedAt.Format(util.DateTimeFormat) + "] old state"
	if task.Comments[0].Text != wantPrefix {
		t.Errorf("comment = %q, want %q", task.Comments[0].Text, wantPrefix)
	}
}

func TestTaskUnpark(t *testing.T) {
	task, err := NewTask("task-unpark", 1, TaskTypeTask, "Resume me")
	if err != nil {
		t.Fatal(err)
	}
	parkedAt := time.Date(2026, 7, 8, 10, 15, 0, 0, time.Local)
	task.ParkedNote = "state"
	task.ParkedAt = &parkedAt

	task.Unpark(false)

	if task.IsParked() {
		t.Fatal("task should not be parked")
	}
	if task.ParkedNote != "" {
		t.Errorf("ParkedNote = %q, want empty", task.ParkedNote)
	}
	if task.ParkedAt != nil {
		t.Errorf("ParkedAt = %v, want nil", task.ParkedAt)
	}
	if len(task.Comments) != 0 {
		t.Errorf("got %d comments, want 0", len(task.Comments))
	}
}

func TestTaskUnparkRollsNoteToComment(t *testing.T) {
	task, err := NewTask("task-unpark-roll", 1, TaskTypeTask, "Resume me")
	if err != nil {
		t.Fatal(err)
	}
	parkedAt := time.Date(2026, 7, 8, 10, 15, 0, 0, time.Local)
	task.ParkedNote = "state to keep"
	task.ParkedAt = &parkedAt

	task.Unpark(true)

	if task.IsParked() {
		t.Fatal("task should not be parked")
	}
	if len(task.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(task.Comments))
	}
	if !strings.Contains(task.Comments[0].Text, "state to keep") {
		t.Errorf("comment %q should contain old note", task.Comments[0].Text)
	}
}

func TestCompleteClearsParkedStateAndRollsNote(t *testing.T) {
	task, err := NewTask("task-complete-parked", 1, TaskTypeTask, "Finish me")
	if err != nil {
		t.Fatal(err)
	}
	parkedAt := time.Date(2026, 7, 8, 10, 15, 0, 0, time.Local)
	task.ParkedNote = "state before completion"
	task.ParkedAt = &parkedAt

	task.Complete()

	if task.IsParked() {
		t.Fatal("completed task should not remain parked")
	}
	if task.Status != StatusDone {
		t.Errorf("Status = %q, want %q", task.Status, StatusDone)
	}
	if len(task.Comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(task.Comments))
	}
	if !strings.Contains(task.Comments[0].Text, "state before completion") {
		t.Errorf("comment %q should contain old note", task.Comments[0].Text)
	}
}
