package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestCaptureSaveErrorSurvivesTimersUntilDraftChanges(t *testing.T) {
	d := polishDriver(t)
	m := d.App()
	d.dispatch(m.captureComposerMessage(m.currentProject, m.taskWorkspaceProjects()))
	d.drain()
	d.dispatch(tea.PasteMsg{Content: "Keep my draft"})
	d.drain()
	d.dispatch(status.ErrorMsg{Err: errors.New("save failed: disk full")})
	d.drain()
	if m.feedback.err == nil {
		t.Fatal("save error expired while draft remained")
	}
	for _, msg := range []tea.Msg{status.StatusClearMsg{ScheduledAt: time.Now().Add(time.Hour)}, tea.KeyPressMsg{Code: tea.KeyLeft}} {
		d.dispatch(msg)
		d.drain()
		if m.feedback.err == nil {
			t.Fatalf("%T cleared the draft error", msg)
		}
	}
	if m.taskComposer.Input.Value() != "Keep my draft" {
		t.Fatal("error changed the draft")
	}
	d.dispatch(help.ShowHelpMsg{Items: []operation.HelpItem{{Key: "e", Description: "Edit", Category: "Tasks"}}, ViewName: "Capture"})
	d.drain()
	d.dispatch(tea.KeyPressMsg{Code: '/', Text: "/"})
	d.dispatch(tea.KeyPressMsg{Code: 'e', Text: "e"})
	d.drain()
	if m.feedback.err == nil {
		t.Fatal("typing in covering Help cleared the draft's error")
	}
	d.dispatch(help.CloseHelpMsg{})
	d.drain()
	d.dispatch(tea.PasteMsg{Content: " corrected"})
	d.drain()
	if m.feedback.err != nil {
		t.Fatal("editing the draft did not retire its error")
	}
	d.dispatch(status.ErrorMsg{Err: errors.New("save failed again")})
	d.drain()
	d.dispatch(tea.KeyPressMsg{Code: tea.KeyEnter})
	d.drain()
	if m.feedback.err != nil {
		t.Fatal("successful save is hidden by the previous error")
	}
}

func TestOrdinaryErrorStillExpiresWithoutADraft(t *testing.T) {
	m := newTestAppModel()
	m.handleViewCoreMessages(status.ErrorMsg{Err: errors.New("nothing to copy")})
	m.handleViewCoreMessages(status.StatusClearMsg{})
	if m.feedback.err != nil {
		t.Fatal("non-draft refusal became persistent")
	}
}
