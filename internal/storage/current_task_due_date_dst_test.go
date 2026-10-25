package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// TestDateOnlyDueDateSurvivesAZoneWithNoMidnight pins the reader against a date
// whose zone skips 00:00. America/Havana starts 2026-03-08 at 01:00, and reading
// that date through a literal local midnight lands at 23:00 on the 7th, which
// DueHasTime then reports as a clock and the writer persists as the wrong day
// plus a due_time nobody set.
func TestDateOnlyDueDateSurvivesAZoneWithNoMidnight(t *testing.T) {
	havana, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	due, err := parseProjectDocumentDueDate("2026-03-08", "", havana)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := time.Date(2026, 3, 8, 1, 0, 0, 0, havana); !due.Equal(want) {
		t.Fatalf("due = %s, want the start of the date %s", due, want)
	}
	task := &models.Task{DueDate: due}
	if task.DueHasTime() {
		t.Fatal("a date-only deadline read back as a timed one")
	}
	record, dueTime := currentTaskBaseFieldsFromModel(task)
	if record.DueDate != "2026-03-08" || dueTime != "" {
		t.Fatalf("written due = %q time %q, want 2026-03-08 with no clock", record.DueDate, dueTime)
	}

	timed, err := parseProjectDocumentDueDate("2026-03-08", "09:15", havana)
	if err != nil {
		t.Fatalf("parse timed: %v", err)
	}
	if got := timed.Format("2006-01-02 15:04"); got != "2026-03-08 09:15" {
		t.Fatalf("timed due = %s, want 2026-03-08 09:15", got)
	}
}
