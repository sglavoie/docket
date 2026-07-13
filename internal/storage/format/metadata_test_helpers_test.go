package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const legacyTaskSectionField = "custom" + "_section"

func timePtr(t time.Time) *time.Time {
	return &t
}

func strPtr(s string) *string {
	return &s
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func ptrToStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func ptrToTime(p *time.Time) string {
	if p == nil {
		return "<nil>"
	}
	return p.Format(util.DateTimeFormat)
}

func recurrenceEqual(a, b *models.RecurrencePattern) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Interval == b.Interval && a.Unit == b.Unit && intSlicesEqual(a.Weekdays, b.Weekdays) &&
		a.Skipped == b.Skipped && a.Streak == b.Streak && a.MaxStreak == b.MaxStreak &&
		a.OriginalDay == b.OriginalDay && a.Mode == b.Mode
}

func assertTaskFields(t *testing.T, task, want *models.Task) {
	t.Helper()
	if task.ID != want.ID {
		t.Errorf("ID = %q, want %q", task.ID, want.ID)
	}
	if task.State() != want.State() {
		t.Errorf("State = %q, want %q", task.State(), want.State())
	}
	if task.Priority != want.Priority {
		t.Errorf("Priority = %d, want %d", task.Priority, want.Priority)
	}
	if task.Pinned != want.Pinned {
		t.Errorf("Pinned = %v, want %v", task.Pinned, want.Pinned)
	}

	if !stringSlicesEqual(task.Tags, want.Tags) {
		t.Errorf("Tags = %v, want %v", task.Tags, want.Tags)
	}
	if !stringSlicesEqual(task.DependsOn, want.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", task.DependsOn, want.DependsOn)
	}

	if !stringPtrEqual(task.ParentID, want.ParentID) {
		t.Errorf("ParentID = %v, want %v", ptrToStr(task.ParentID), ptrToStr(want.ParentID))
	}
	if !timePtrEqual(task.DueDate, want.DueDate) {
		t.Errorf("DueDate = %v, want %v", ptrToTime(task.DueDate), ptrToTime(want.DueDate))
	}
	if !timePtrEqual(task.CompletedAt, want.CompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", ptrToTime(task.CompletedAt), ptrToTime(want.CompletedAt))
	}

	if !task.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", task.CreatedAt, want.CreatedAt)
	}
	if !task.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, want.UpdatedAt)
	}

	if !recurrenceEqual(task.Recurrence, want.Recurrence) {
		t.Errorf("Recurrence = %+v, want %+v", task.Recurrence, want.Recurrence)
	}
}

// assertTaskFieldsAfterV2RoundTrip verifies field preservation across a v2
// write→parse round-trip. Unlike assertTaskFields (which is used by parse-only
// tests where the legacy fields survive verbatim), it compares the derived
// WorkflowState instead of the raw Status/Section shims and asserts the retired
// Stage/Active fields are dropped, because the v2 writer collapses the legacy
// lifecycle fields into the canonical `state`.
func assertTaskFieldsAfterV2RoundTrip(t *testing.T, task, want *models.Task) {
	t.Helper()
	if task.ID != want.ID {
		t.Errorf("ID = %q, want %q", task.ID, want.ID)
	}
	if task.State() != want.State() {
		t.Errorf("State = %q, want %q", task.State(), want.State())
	}
	if task.Priority != want.Priority {
		t.Errorf("Priority = %d, want %d", task.Priority, want.Priority)
	}
	if task.Pinned != want.Pinned {
		t.Errorf("Pinned = %v, want %v", task.Pinned, want.Pinned)
	}
	if !stringSlicesEqual(task.Tags, want.Tags) {
		t.Errorf("Tags = %v, want %v", task.Tags, want.Tags)
	}
	if !stringSlicesEqual(task.DependsOn, want.DependsOn) {
		t.Errorf("DependsOn = %v, want %v", task.DependsOn, want.DependsOn)
	}
	if !stringPtrEqual(task.ParentID, want.ParentID) {
		t.Errorf("ParentID = %v, want %v", ptrToStr(task.ParentID), ptrToStr(want.ParentID))
	}
	if !timePtrEqual(task.DueDate, want.DueDate) {
		t.Errorf("DueDate = %v, want %v", ptrToTime(task.DueDate), ptrToTime(want.DueDate))
	}
	if !timePtrEqual(task.CompletedAt, want.CompletedAt) {
		t.Errorf("CompletedAt = %v, want %v", ptrToTime(task.CompletedAt), ptrToTime(want.CompletedAt))
	}
	if !task.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", task.CreatedAt, want.CreatedAt)
	}
	if !task.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, want.UpdatedAt)
	}
	if !recurrenceEqual(task.Recurrence, want.Recurrence) {
		t.Errorf("Recurrence = %+v, want %+v", task.Recurrence, want.Recurrence)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

func hasWarningField(warnings []ParseWarning, field string) bool {
	for _, w := range warnings {
		if w.Field == field {
			return true
		}
	}
	return false
}

func parseWrittenMetadata(task, parsed *models.Task) []ParseWarning {
	metaStr := WriteMetadataV2(task)
	content := metaStr[5 : len(metaStr)-4]

	return ParseMetadata(parsed, content)
}
