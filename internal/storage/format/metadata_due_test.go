package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestMetadataDueRoundTrip(t *testing.T) {
	dateOnly := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	dateWithTime := time.Date(2025, 6, 15, 17, 30, 0, 0, time.Local)

	t.Run("HasTime=false round-trips as YYYY-MM-DD", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &dateOnly, HasTime: false}
		parsed := &models.Task{WorkflowState: models.StateReady}
		parseWrittenMetadata(task, parsed)

		if parsed.HasTime {
			t.Errorf("HasTime = true, want false")
		}
		if !timePtrEqual(parsed.DueDate, &dateOnly) {
			t.Errorf("DueDate = %v, want %v", ptrToTime(parsed.DueDate), dateOnly.Format(util.DateFormat))
		}
	})

	t.Run("HasTime=true round-trips as YYYY-MM-DDTHH:MM", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &dateWithTime, HasTime: true}
		parsed := &models.Task{WorkflowState: models.StateReady}
		parseWrittenMetadata(task, parsed)

		if !parsed.HasTime {
			t.Errorf("HasTime = false, want true")
		}
		if !timePtrEqual(parsed.DueDate, &dateWithTime) {
			t.Errorf("DueDate = %v, want %v", ptrToTime(parsed.DueDate), dateWithTime.Format(util.DateTimeDueFormat))
		}
	})

	t.Run("nil DueDate writes no due field", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady}
		meta := WriteMetadataV2(task)
		if strings.Contains(meta, "due:") {
			t.Errorf("expected no 'due:' in metadata, got: %s", meta)
		}
	})

	t.Run("HasTime=false writes YYYY-MM-DD format", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &dateOnly, HasTime: false}
		meta := WriteMetadataV2(task)
		if !strings.Contains(meta, "due: 2025-06-15") {
			t.Errorf("expected 'due: 2025-06-15' in metadata, got: %s", meta)
		}
		if strings.Contains(meta, "T") {
			t.Errorf("expected no 'T' in due field for date-only, got: %s", meta)
		}
	})

	t.Run("HasTime=true writes YYYY-MM-DDTHH:MM format", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &dateWithTime, HasTime: true}
		meta := WriteMetadataV2(task)
		if !strings.Contains(meta, "due: 2025-06-15T17:30") {
			t.Errorf("expected 'due: 2025-06-15T17:30' in metadata, got: %s", meta)
		}
	})

	t.Run("HasTime cleared when re-parsed from date-only format", func(t *testing.T) {
		// If a task has a due with clock midnight (00:00) but HasTime=false,
		// it should survive the round-trip with HasTime still false.
		midnight := time.Date(2025, 3, 10, 0, 0, 0, 0, time.Local)
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &midnight, HasTime: false}
		parsed := &models.Task{WorkflowState: models.StateReady}
		parseWrittenMetadata(task, parsed)

		if parsed.HasTime {
			t.Errorf("HasTime = true for midnight date-only, want false")
		}
		if !timePtrEqual(parsed.DueDate, &midnight) {
			t.Errorf("DueDate = %v, want %v", ptrToTime(parsed.DueDate), midnight.Format(util.DateFormat))
		}
	})
}
