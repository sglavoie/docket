package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/status"
)

func TestShellFeedbackReadingTimeAndSupersession(t *testing.T) {
	previous := status.TickScheduler
	t.Cleanup(func() { status.TickScheduler = previous })
	var delay time.Duration
	status.TickScheduler = func(d time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
		delay = d
		return func() tea.Msg { return fire(time.Now().Add(d)) }
	}
	model := newTestAppModel()
	_, first := model.handleClipboardMessages(TaskCopiedMsg{Count: 4, Hidden: 1, TitleAndDesc: true})
	if want := status.StatusDuration(model.feedback.message); delay != want || delay <= 2*time.Second {
		t.Fatalf("clipboard feedback lasts %v, want %v and more than two seconds", delay, want)
	}
	oldClear := first()
	_, second := model.handleViewCoreMessages(status.InfoStatusMsg("Saved"))
	if delay != status.StatusDuration("Saved") {
		t.Fatalf("info feedback lasts %v", delay)
	}
	model.handleViewCoreMessages(oldClear)
	if model.feedback.message != "Saved" {
		t.Fatal("older timer dismissed newer feedback")
	}
	model.handleViewCoreMessages(second())
	if model.feedback.message != "" {
		t.Fatal("current timer did not dismiss feedback")
	}

	model.setUndoableStatus("Deleted · u undo", func() tea.Cmd { return nil })
	if delay != UndoToastTimeout {
		t.Fatalf("undo timer changed to %v, want %v", delay, UndoToastTimeout)
	}
	model.handleViewCoreMessages(status.StatusClearMsg{ScheduledAt: time.Now()})
	if model.feedback.undoCb == nil || model.feedback.message == "" {
		t.Fatal("plain timer dismissed a live undo toast")
	}
}
