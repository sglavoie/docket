package tui

import (
	"strings"
	"testing"
)

func TestCloseInboxProcessorMessageOffersNextActions(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	msg := CloseInboxProcessorMsg{ProcessedCount: 3, SkippedCount: 1}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	got := app.statusMessage
	for _, want := range []string{"3 triaged", "1 skipped", "Planner Today", "or Next"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in status, got %q", want, got)
		}
	}
}

func TestCloseInboxProcessorSkippedOnlyOffersNext(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	handled, cmd := app.handleInboxMessages(CloseInboxProcessorMsg{SkippedCount: 2})
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	if got := app.statusMessage; !strings.Contains(got, "2 skipped") || !strings.Contains(got, "Planner Next") {
		t.Errorf("expected skipped count and Planner Next in status, got %q", got)
	}
}

func TestCloseGlobalInboxProcessorMessageOffersNextActions(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewGlobalInboxProcessor)

	handled, cmd := app.handleInboxMessages(CloseGlobalInboxProcessorMsg{ProcessedCount: 5, SkippedCount: 2})
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	got := app.statusMessage
	for _, want := range []string{"5 triaged", "2 skipped", "Planner Today", "or Next"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in status, got %q", want, got)
		}
	}
}

func TestCloseGlobalInboxProcessorSkippedOnlyOffersNext(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewGlobalInboxProcessor)

	handled, cmd := app.handleInboxMessages(CloseGlobalInboxProcessorMsg{SkippedCount: 3})
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	if got := app.statusMessage; !strings.Contains(got, "3 skipped") || !strings.Contains(got, "Planner Next") {
		t.Errorf("expected skipped count and Planner Next in status, got %q", got)
	}
}
