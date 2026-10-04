package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionCommand, Command: "Last message"}, ObserveOptions{}); err != nil {
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
	if len(messages) != 20 || messages[0] != "Saved 24" || messages[19] != "Saved 5" {
		t.Fatalf("unexpected retained history: %v", messages)
	}
	before := m.captureFooterFlashProbe()
	m.feedback.appendNote("Check-in is unset")
	m.rememberFeedbackSince(before)
	if len(m.feedback.history) != 20 || m.feedback.latestMessage() != "Saved 24 · Check-in is unset" {
		t.Fatalf("amendment created another event: %v", m.feedback.feedbackSnapshot())
	}
	if messages[0] != "Saved 24" {
		t.Fatal("viewer snapshot changed with live feedback")
	}
	m.Update(status.StatusClearMsg{})
	m.Update(struct{}{})
	if len(m.feedback.history) != 20 {
		t.Fatal("expiry or redraw recorded feedback")
	}
	m.Update(status.InfoStatusMsg("Saved 24 · Check-in is unset"))
	if m.feedback.feedbackSnapshot()[1] != m.feedback.latestMessage() {
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
	if len(messages) != 2 || messages[0] != "Copied title" || messages[1] != "Cannot save this task" {
		t.Fatalf("error was lost: %v", messages)
	}
}

func (c *feedbackController) latestMessage() string {
	if len(c.history) == 0 {
		return ""
	}
	return c.history[len(c.history)-1].text
}
