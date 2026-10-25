package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestLastMessageSurvivesExpiryAndReadsLatestFeedback(t *testing.T) {
	m := newTestAppModel()
	m.Update(status.InfoStatusMsg("Saved a long and useful explanation"))
	if m.feedback.latestMessage() != "Saved a long and useful explanation" {
		t.Fatalf("recall=%q", m.feedback.latestMessage())
	}
	m.Update(status.StatusClearMsg{})
	if m.feedback.message != "" || m.feedback.latestMessage() == "" {
		t.Fatal("expiry cleared recall")
	}
	m.Update(status.ErrorMsg{Err: errors.New("Cannot save this task")})
	if m.feedback.latestMessage() != "Cannot save this task" {
		t.Fatalf("error recall=%q", m.feedback.latestMessage())
	}
	m.Update(status.StatusClearMsg{})
	if m.feedback.latestMessage() != "Cannot save this task" {
		t.Fatal("clearing error cleared recall")
	}
}

func TestLastMessageMenuOpensAndReturnsWithoutChangingOutline(t *testing.T) {
	driver, out := menuBarDriver(t)
	m := driver.App()
	m.Update(status.InfoStatusMsg("Remember this explanation"))
	m.Update(status.StatusClearMsg{})
	cursor := out.Cursor
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionCommand, Command: "Recent Messages"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if m.currentView() != presentation.ViewLastMessage || !strings.Contains(m.View().Content, "Remember this explanation") {
		t.Fatalf("viewer missing: %s", m.View().Content)
	}
	m.Update(syntheticSpecialKey(tea.KeyEscape))
	if m.currentView() != presentation.ViewCrossProjectOutliner || out.Cursor != cursor {
		t.Fatal("close did not restore outline")
	}
}

func TestLastMessageSearchOwnsInputAndEscBeforeReturningToOutline(t *testing.T) {
	driver, out := menuBarDriver(t)
	var copied string
	withFakeClipboard(t, nil, &copied)
	m := driver.App()
	m.Update(status.ErrorMsg{Err: errors.New("Disk unavailable")})
	m.Update(status.InfoStatusMsg("Saved task"))
	m.Update(status.StatusClearMsg{})
	cursor := out.Cursor
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionCommand, Command: "Recent Messages"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"/", "d", "i", "s", "k", "enter"} {
		if err := driver.SendKey(key); err != nil {
			t.Fatal(err)
		}
	}
	plain := ansi.Strip(m.View().Content)
	if m.currentView() != presentation.ViewLastMessage || !strings.Contains(plain, "Disk unavailable") || strings.Contains(plain, "Saved task") {
		t.Fatalf("search did not retain the message view:\n%s", m.View().Content)
	}
	if err := driver.SendKey("ctrl+y"); err != nil {
		t.Fatal(err)
	}
	driver.drain()
	if !strings.Contains(copied, "Disk unavailable") || strings.Contains(copied, "Saved task") ||
		!strings.Contains(m.View().Content, "Copied 1 matching message") || out.Cursor != cursor {
		t.Fatalf("matching copy did not stay in the viewer: clipboard=%q\n%s", copied, m.View().Content)
	}
	if err := driver.SendKey("esc"); err != nil {
		t.Fatal(err)
	}
	if m.currentView() != presentation.ViewLastMessage || !strings.Contains(m.View().Content, "Saved task") {
		t.Fatal("first Esc did not restore the original message")
	}
	if err := driver.SendKey("esc"); err != nil {
		t.Fatal(err)
	}
	if m.currentView() != presentation.ViewCrossProjectOutliner || out.Cursor != cursor {
		t.Fatal("second Esc did not return to the original outline cursor")
	}
}

func TestLastMessageRemembersViewFeedbackAndIgnoresAdvisories(t *testing.T) {
	driver, out := menuBarDriver(t)
	m := driver.App()
	before := m.captureFooterFlashProbe()
	out.SetTransientStatus("Task is hidden by these filters")
	m.rememberFeedbackSince(before)
	if m.feedback.latestMessage() != "Task is hidden by these filters" {
		t.Fatalf("view feedback=%q", m.feedback.latestMessage())
	}
	before = m.captureFooterFlashProbe()
	out.ClearStatus()
	m.feedback.outstandingConflicts = 2
	m.rememberFeedbackSince(before)
	if m.feedback.latestMessage() != "Task is hidden by these filters" {
		t.Fatal("expiry or advisory replaced recall")
	}
	before = m.captureFooterFlashProbe()
	m.setInfoStatus("Saved")
	m.feedback.appendNote("Check-in is unset")
	m.rememberFeedbackSince(before)
	if m.feedback.latestMessage() != "Saved · Check-in is unset" {
		t.Fatalf("amended feedback=%q", m.feedback.latestMessage())
	}
}

func TestLastMessageHistoryRetainsTwentyEventsAndAmendsDetails(t *testing.T) {
	m := newTestAppModel()
	for i := range 25 {
		m.Update(status.InfoStatusMsg(fmt.Sprintf("Saved %d", i)))
	}
	messages := m.feedback.feedbackSnapshot()
	if len(messages) != 20 || messages[0].Text != "Saved 24" || messages[19].Text != "Saved 5" {
		t.Fatalf("unexpected retained history: %v", messages)
	}
	before := m.captureFooterFlashProbe()
	m.feedback.appendNote("Check-in is unset")
	m.rememberFeedbackSince(before)
	if len(m.feedback.history) != 20 || m.feedback.latestMessage() != "Saved 24 · Check-in is unset" {
		t.Fatalf("amendment created another event: %v", m.feedback.feedbackSnapshot())
	}
	if messages[0].Text != "Saved 24" {
		t.Fatal("viewer snapshot changed with live feedback")
	}
	m.Update(status.StatusClearMsg{})
	m.Update(struct{}{})
	if len(m.feedback.history) != 20 {
		t.Fatal("expiry or redraw recorded feedback")
	}
	m.Update(status.InfoStatusMsg("Saved 24 · Check-in is unset"))
	if m.feedback.feedbackSnapshot()[1].Text != m.feedback.latestMessage() {
		t.Fatal("distinct events with identical text were collapsed")
	}
	if len(newTestAppModel().feedback.feedbackSnapshot()) != 0 {
		t.Fatal("history leaked into another session")
	}
}

func TestLastMessageHistoryKeepsErrorAfterRoutineSuccess(t *testing.T) {
	m := newTestAppModel()
	m.Update(status.ErrorMsg{Err: errors.New("Cannot save this task")})
	m.Update(status.StatusClearMsg{})
	m.Update(status.InfoStatusMsg("Copied title"))
	messages := m.feedback.feedbackSnapshot()
	if len(messages) != 2 || messages[0].Text != "Copied title" || messages[1].Text != "Cannot save this task" {
		t.Fatalf("error was lost: %v", messages)
	}
}

func (c *feedbackController) latestMessage() string {
	if len(c.history) == 0 {
		return ""
	}
	return c.history[len(c.history)-1].Text
}

func TestFeedbackHistoryAmendmentPreservesOriginalTimestamp(t *testing.T) {
	var feedback feedbackController
	feedback.rememberFeedback("Saved", "shell", 1)
	original := feedback.feedbackSnapshot()[0]
	if original.At.IsZero() || original.Kind != "Feedback" {
		t.Fatalf("missing metadata: %+v", original)
	}
	feedback.rememberFeedback("Saved · extra detail", "shell", 1)
	amended := feedback.feedbackSnapshot()[0]
	if !amended.At.Equal(original.At) || original.Text != "Saved" {
		t.Fatal("amendment changed time or original snapshot")
	}
	feedback.rememberFeedback("Failed", "error", 0)
	if got := feedback.feedbackSnapshot()[0]; got.Kind != "Error" || got.At.IsZero() {
		t.Fatalf("error metadata: %+v", got)
	}
}
