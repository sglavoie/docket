package models

import "testing"

func TestTaskStatusIsValid(t *testing.T) {
	valid := []TaskStatus{StatusTodo, StatusInProgress, StatusReview, StatusDone, StatusBlocked, StatusCanceled}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []TaskStatus{"unknown", "", "pending", "DONE"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestTaskStatusIsComplete(t *testing.T) {
	complete := []TaskStatus{StatusDone, StatusCanceled}
	for _, s := range complete {
		if !s.IsComplete() {
			t.Errorf("expected %q to be complete", s)
		}
	}

	notComplete := []TaskStatus{StatusTodo, StatusInProgress, StatusReview, StatusBlocked}
	for _, s := range notComplete {
		if s.IsComplete() {
			t.Errorf("expected %q to not be complete", s)
		}
	}
}

func TestTaskStatusIsActive(t *testing.T) {
	active := []TaskStatus{StatusTodo, StatusInProgress, StatusReview, StatusBlocked}
	for _, s := range active {
		if !s.IsActive() {
			t.Errorf("expected %q to be active", s)
		}
	}

	notActive := []TaskStatus{StatusDone, StatusCanceled}
	for _, s := range notActive {
		if s.IsActive() {
			t.Errorf("expected %q to not be active", s)
		}
	}
}

func TestTaskStatusIcon(t *testing.T) {
	tests := []struct {
		status TaskStatus
		icon   string
	}{
		{StatusTodo, "○"},
		{StatusInProgress, "◐"},
		{StatusReview, "◑"},
		{StatusDone, "●"},
		{StatusBlocked, "◈"},
		{StatusCanceled, "✕"},
		{TaskStatus("unknown"), "?"},
	}

	for _, tt := range tests {
		if got := tt.status.Icon(); got != tt.icon {
			t.Errorf("Icon() for %q: expected %q, got %q", tt.status, tt.icon, got)
		}
	}
}

func TestParseTaskStatusExtended(t *testing.T) {
	tests := []struct {
		input string
		want  TaskStatus
		ok    bool
	}{
		{"todo", StatusTodo, true},
		{"in_progress", StatusInProgress, true},
		{"review", StatusReview, true},
		{"done", StatusDone, true},
		{"blocked", StatusBlocked, true},
		{"canceled", StatusCanceled, true},
		{"unknown", TaskStatus("unknown"), false},
		{"", TaskStatus(""), false},
		{"DONE", TaskStatus("DONE"), false},
		{"pending", TaskStatus("pending"), false},
	}

	for _, tt := range tests {
		got, ok := ParseTaskStatus(tt.input)
		if ok != tt.ok {
			t.Errorf("ParseTaskStatus(%q): expected ok=%v, got ok=%v", tt.input, tt.ok, ok)
		}
		if got != tt.want {
			t.Errorf("ParseTaskStatus(%q): expected %q, got %q", tt.input, tt.want, got)
		}
	}
}

func TestAllTaskStatuses(t *testing.T) {
	statuses := AllTaskStatuses()
	if len(statuses) != 6 {
		t.Errorf("expected 6 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if !s.IsValid() {
			t.Errorf("AllTaskStatuses returned invalid status %q", s)
		}
	}
}

func TestIsGTDSection(t *testing.T) {
	gtdSections := []string{SectionNext, SectionInbox, SectionWaiting, SectionSomeday, SectionReview}
	for _, s := range gtdSections {
		if !IsGTDSection(s) {
			t.Errorf("expected %q to be a GTD section", s)
		}
	}

	nonGTD := []string{"", "Custom", "active", "done"}
	for _, s := range nonGTD {
		if IsGTDSection(s) {
			t.Errorf("expected %q to not be a GTD section", s)
		}
	}
}
