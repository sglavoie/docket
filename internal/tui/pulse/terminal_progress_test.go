package pulse

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTerminalProgressExistsOnlyWhileTodayHoldsWork(t *testing.T) {
	if got := TerminalProgress(WorkspacePulse{Done: 4}); got != nil {
		t.Fatalf("empty Today drew a bar: %+v", got)
	}
	got := TerminalProgress(WorkspacePulse{Today: 3, Done: 1})
	if got == nil || got.State != tea.ProgressBarDefault || got.Value != 25 {
		t.Fatalf("progress = %+v, want default 25", got)
	}
	if got := TerminalProgress(WorkspacePulse{Today: 2}); got == nil || got.Value != 0 {
		t.Fatalf("untouched Today = %+v, want 0", got)
	}
}

func TestTerminalProgressWarnsWhileWorkIsOverdue(t *testing.T) {
	got := TerminalProgress(WorkspacePulse{Today: 1, Done: 1, Overdue: 1})
	if got == nil || got.State != tea.ProgressBarWarning || got.Value != 50 {
		t.Fatalf("progress = %+v, want warning 50", got)
	}
}
