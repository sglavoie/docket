package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestLastMessageSurvivesExpiryAndReadsLatestFeedback(t *testing.T) {
	m := newTestAppModel()
	m.Update(status.InfoStatusMsg("Saved a long and useful explanation"))
	if m.feedback.lastMessage != "Saved a long and useful explanation" {
		t.Fatalf("recall=%q", m.feedback.lastMessage)
	}
	m.Update(status.StatusClearMsg{})
	if m.feedback.message != "" || m.feedback.lastMessage == "" {
		t.Fatal("expiry cleared recall")
	}
	m.Update(status.ErrorMsg{Err: errors.New("Cannot save this task")})
	if m.feedback.lastMessage != "Cannot save this task" {
		t.Fatalf("error recall=%q", m.feedback.lastMessage)
	}
	m.Update(status.StatusClearMsg{})
	if m.feedback.lastMessage != "Cannot save this task" {
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
	if m.feedback.lastMessage != "Task is hidden by these filters" {
		t.Fatalf("view feedback=%q", m.feedback.lastMessage)
	}
	before = m.captureFooterFlashProbe()
	out.ClearStatus()
	m.feedback.outstandingConflicts = 2
	m.rememberFeedbackSince(before)
	if m.feedback.lastMessage != "Task is hidden by these filters" {
		t.Fatal("expiry or advisory replaced recall")
	}
	before = m.captureFooterFlashProbe()
	m.setInfoStatus("Saved")
	m.feedback.appendNote("Check-in is unset")
	m.rememberFeedbackSince(before)
	if m.feedback.lastMessage != "Saved · Check-in is unset" {
		t.Fatalf("amended feedback=%q", m.feedback.lastMessage)
	}
}
