package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestMetadataDueRoundTrip(t *testing.T) {
	dateOnly := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	t.Run("date-only due round-trips as YYYY-MM-DD", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &dateOnly}
		parsed := &models.Task{WorkflowState: models.StateReady}
		parseWrittenMetadata(task, parsed)

		if !timePtrEqual(parsed.DueDate, &dateOnly) {
			t.Errorf("DueDate = %v, want %v", ptrToTime(parsed.DueDate), dateOnly)
		}
	})

	t.Run("nil DueDate writes no due field", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady}
		meta := WriteMetadataV2(task)
		if strings.Contains(meta, "due:") {
			t.Errorf("expected no 'due:' in metadata, got: %s", meta)
		}
	})

	t.Run("due writes YYYY-MM-DD format", func(t *testing.T) {
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &dateOnly}
		meta := WriteMetadataV2(task)
		if !strings.Contains(meta, "due: 2025-06-15") {
			t.Errorf("expected 'due: 2025-06-15' in metadata, got: %s", meta)
		}
		if strings.Contains(meta, "T") {
			t.Errorf("expected no 'T' in due field for date-only, got: %s", meta)
		}
	})

	t.Run("midnight date remains date-only", func(t *testing.T) {
		midnight := time.Date(2025, 3, 10, 0, 0, 0, 0, time.Local)
		task := &models.Task{WorkflowState: models.StateReady, DueDate: &midnight}
		parsed := &models.Task{WorkflowState: models.StateReady}
		parseWrittenMetadata(task, parsed)

		if !timePtrEqual(parsed.DueDate, &midnight) {
			t.Errorf("DueDate = %v, want %v", ptrToTime(parsed.DueDate), midnight)
		}
	})
}
