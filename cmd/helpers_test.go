package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestFormatTaskDueDate_DateOnly(t *testing.T) {
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)
	task := &models.Task{DueDate: &due, HasTime: false}
	got := formatTaskDueDate(task)
	want := "2026-06-15"
	if got != want {
		t.Errorf("formatTaskDueDate(date-only) = %q, want %q", got, want)
	}
}

func TestFormatTaskDueDate_WithTime(t *testing.T) {
	due := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)
	task := &models.Task{DueDate: &due, HasTime: true}
	got := formatTaskDueDate(task)
	want := "2026-06-15 14:30"
	if got != want {
		t.Errorf("formatTaskDueDate(with-time) = %q, want %q", got, want)
	}
}

func TestFormatTaskDueDate_NilDueDate(t *testing.T) {
	task := &models.Task{DueDate: nil}
	got := formatTaskDueDate(task)
	want := "none"
	if got != want {
		t.Errorf("formatTaskDueDate(nil) = %q, want %q", got, want)
	}
}
