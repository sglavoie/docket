package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/status"
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

func TestRefusalWrapsRecoveryAndKeepsTimeToAct(t *testing.T) {
	previous := status.TickScheduler
	t.Cleanup(func() { status.TickScheduler = previous })
	var delay time.Duration
	status.TickScheduler = func(d time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
		delay = d
		return func() tea.Msg { return fire(time.Now().Add(d)) }
	}
	model := newTestAppModel()
	model.viewNav.width = 60
	message := "Cannot apply outline: the workspace changed while you were editing. Your draft is kept. Reopen the outline to review the newer tasks."
	model.setWarningStatus(message)
	clear := model.statusReadingTimeout()
	feedback := ansi.Strip(model.footerFeedbackLine())
	if lipgloss.Height(feedback) != 3 || !strings.Contains(feedback, "newer tasks.") {
		t.Fatalf("refusal lost its recovery action: %q", feedback)
	}
	if delay <= 6*time.Second || delay > 12*time.Second {
		t.Fatalf("wrapped refusal lasts %v", delay)
	}
	for _, line := range strings.Split(feedback, "\n") {
		if lipgloss.Width(line) != 60 {
			t.Fatalf("refusal line exceeds its budget: %q", line)
		}
	}
	model.setInfoStatus("Saved")
	model.handleViewCoreMessages(clear())
	if model.feedback.message != "Saved" || lipgloss.Height(model.footerFeedbackLine()) != 1 {
		t.Fatal("old refusal timer or wrapping changed the subsequent success")
	}
	model.feedback.err = errors.New(strings.Repeat("A long diagnostic with more detail. ", 20))
	if feedback = ansi.Strip(model.footerFeedbackLine()); lipgloss.Height(feedback) != 3 || !strings.Contains(feedback, "Recent messages for full details") {
		t.Fatalf("long error has no bounded route to its full text: %q", feedback)
	}
}
