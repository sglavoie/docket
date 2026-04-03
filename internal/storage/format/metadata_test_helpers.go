package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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
		a.OriginalDay == b.OriginalDay
}

func assertTaskFields(t *testing.T, task, want *models.Task) {
	t.Helper()
	if task.ID != want.ID {
		t.Errorf("ID = %q, want %q", task.ID, want.ID)
	}
	if task.Status != want.Status {
		t.Errorf("Status = %q, want %q", task.Status, want.Status)
	}
	if task.Priority != want.Priority {
		t.Errorf("Priority = %d, want %d", task.Priority, want.Priority)
	}
	if task.Section != want.Section {
		t.Errorf("Section = %q, want %q", task.Section, want.Section)
	}
	if task.CustomSection != want.CustomSection {
		t.Errorf("CustomSection = %q, want %q", task.CustomSection, want.CustomSection)
	}
	if task.Pinned != want.Pinned {
		t.Errorf("Pinned = %v, want %v", task.Pinned, want.Pinned)
	}
	if task.Active != want.Active {
		t.Errorf("Active = %v, want %v", task.Active, want.Active)
	}
	if task.Stage != want.Stage {
		t.Errorf("Stage = %q, want %q", task.Stage, want.Stage)
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

func parseWrittenMetadata(task, parsed *models.Task) []ParseWarning {
	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]

	return ParseMetadata(parsed, content)
}
