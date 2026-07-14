package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestStateForPrefixKey(t *testing.T) {
	tests := []struct {
		key    string
		want   models.WorkflowState
		wantOK bool
	}{
		{"s", models.StateInProgress, true},
		{"d", models.StateDone, true},
		{"b", models.StateWaiting, true},
		{"o", models.StateInbox, true},
		{"x", models.StateCanceled, true},
		{"r", models.StateReview, true},
		{"S", models.StatePaused, true},
		{"m", models.StateSomeday, true},
		{"z", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := StateForPrefixKey(tt.key)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("StateForPrefixKey(%q) = (%v, %v), want (%v, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestStateVerb(t *testing.T) {
	tests := []struct {
		state models.WorkflowState
		want  string
	}{
		{models.StateInProgress, "started"},
		{models.StateDone, "completed"},
		{models.StateWaiting, "set to waiting"},
		{models.StateInbox, "reopened"},
		{models.StateCanceled, "canceled"},
		{models.StateReview, "sent to review"},
		{models.StatePaused, "paused"},
		{models.StateSomeday, "set to someday"},
		{models.StateReady, "set to ready"},
		{models.StateNext, "queued as next"},
	}
	for _, tt := range tests {
		got := StateVerb(tt.state)
		if got != tt.want {
			t.Errorf("StateVerb(%v) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestDueDateDaysForPrefixKey(t *testing.T) {
	tests := []struct {
		key       string
		wantDays  int
		wantClear bool
		wantOK    bool
	}{
		{"t", 0, false, true},
		{"m", 1, false, true},
		{"w", 7, false, true},
		{"n", 30, false, true},
		{"0", 0, true, true},
		{"z", 0, false, false},
		{"", 0, false, false},
	}
	for _, tt := range tests {
		days, isClear, ok := DueDateDaysForPrefixKey(tt.key)
		if days != tt.wantDays || isClear != tt.wantClear || ok != tt.wantOK {
			t.Errorf("DueDateDaysForPrefixKey(%q) = (%d, %v, %v), want (%d, %v, %v)",
				tt.key, days, isClear, ok, tt.wantDays, tt.wantClear, tt.wantOK)
		}
	}
}

func TestPriorityForPrefixKey(t *testing.T) {
	tests := []struct {
		key    string
		want   int
		wantOK bool
	}{
		{"1", 1, true},
		{"2", 2, true},
		{"3", 3, true},
		{"4", 4, true},
		{"5", 5, true},
		{"0", 0, false},
		{"6", 0, false},
		{"a", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := PriorityForPrefixKey(tt.key)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("PriorityForPrefixKey(%q) = (%d, %v), want (%d, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestDueDateFromDays(t *testing.T) {
	t.Run("returns nil for clear", func(t *testing.T) {
		d, desc := dueDateFromDays(0, true)
		if d != nil {
			t.Error("expected nil for isClear=true")
		}
		if desc != "cleared" {
			t.Errorf("got desc %q, want %q", desc, "cleared")
		}
	})

	t.Run("returns start of day", func(t *testing.T) {
		d, _ := dueDateFromDays(1, false)
		if d == nil {
			t.Fatal("expected non-nil date")
		}
		// The time component should be midnight (start of day)
		if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
			t.Errorf("expected start-of-day (00:00:00), got %02d:%02d:%02d",
				d.Hour(), d.Minute(), d.Second())
		}
	})

	t.Run("correct day offset", func(t *testing.T) {
		d, _ := dueDateFromDays(7, false)
		if d == nil {
			t.Fatal("expected non-nil date")
		}
		expected := time.Now().AddDate(0, 0, 7)
		if d.Year() != expected.Year() || d.Month() != expected.Month() || d.Day() != expected.Day() {
			t.Errorf("expected date %v, got %v", expected.Format("2006-01-02"), d.Format("2006-01-02"))
		}
	})
}

func TestSectionForMoveKey(t *testing.T) {
	tests := []struct {
		key    string
		want   string
		wantOK bool
	}{
		{"a", "", true},
		{"n", models.SectionNext, true},
		{"i", models.SectionInbox, true},
		{"w", models.SectionWaiting, true},
		{"s", models.SectionSomeday, true},
		{"r", models.SectionReview, true},
		{"z", "", false},
	}
	for _, tt := range tests {
		got, ok := sectionForMoveKey(tt.key)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("sectionForMoveKey(%q) = (%q, %v), want (%q, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}
