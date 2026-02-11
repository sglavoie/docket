package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestStatusForPrefixKey(t *testing.T) {
	tests := []struct {
		key    string
		want   models.TaskStatus
		wantOK bool
	}{
		{"s", models.StatusInProgress, true},
		{"d", models.StatusDone, true},
		{"b", models.StatusBlocked, true},
		{"o", models.StatusTodo, true},
		{"x", models.StatusCanceled, true},
		{"r", models.StatusReview, true},
		{"z", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := StatusForPrefixKey(tt.key)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("StatusForPrefixKey(%q) = (%v, %v), want (%v, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestStatusVerb(t *testing.T) {
	tests := []struct {
		status models.TaskStatus
		want   string
	}{
		{models.StatusInProgress, "started"},
		{models.StatusDone, "completed"},
		{models.StatusBlocked, "blocked"},
		{models.StatusTodo, "reopened"},
		{models.StatusCanceled, "canceled"},
		{models.StatusReview, "sent to review"},
	}
	for _, tt := range tests {
		got := StatusVerb(tt.status)
		if got != tt.want {
			t.Errorf("StatusVerb(%v) = %q, want %q", tt.status, got, tt.want)
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
