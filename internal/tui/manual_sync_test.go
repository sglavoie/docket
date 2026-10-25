package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/syncexchange"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func TestManualSyncChordExchangesBothDirections(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	retitleOnDisk(t, pair.laptop, "Incoming from phone")
	exchangeOnce(t, pair.laptop)
	if _, err := pair.desk.Service().Capture(app.CaptureRequest{
		ProjectID: pair.desk.State().SystemUnfiledProject().ID, Title: "Outgoing from desktop",
	}); err != nil {
		t.Fatal(err)
	}

	cmd := pressFilterChord(t, model, "gs")
	if !model.automaticSync.running || model.feedback.message != "Syncing…" {
		t.Fatalf("gs did not start an exchange with progress: %q", model.feedback.message)
	}
	runSession2AppCommand(t, model, cmd)
	if model.automaticSync.running || model.automaticSync.manual {
		t.Fatal("manual pass did not release its state")
	}
	if !strings.Contains(model.feedback.message, "Sync: 1 published, 1 imported") {
		t.Fatalf("missing exchange result: %q", model.feedback.message)
	}
	project := model.domain.projects[pair.desk.State().SystemUnfiledProject().ID]
	if project.Tasks[0].Title != "Incoming from phone" || model.externalReload.owed {
		t.Fatal("manual exchange did not refresh the desktop view")
	}
	exchangeOnce(t, pair.laptop)
	assertSyncTaskTitle(t, pair.laptop, "Outgoing from desktop")
	assertSyncTaskTitle(t, pair.desk, "Incoming from phone")

	menu := menuWithID(t, resolveOutlinerMenus(t, model, chrome.ModeAll), outliner.MenuIDGo)
	for _, entry := range menu.Entries {
		if entry.OperationID == operation.OperationCoreSyncNow && entry.Key == "gs" {
			return
		}
	}
	t.Fatal("Go menu lacks Sync Now with the registered shortcut")
}

func TestManualSyncRefreshesChangesAlreadyExchangedOutsideTUI(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	retitleOnDisk(t, pair.laptop, "Already exchanged")
	exchangeOnce(t, pair.laptop)
	exchangeOnce(t, pair.desk)
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	project := model.domain.projects[pair.desk.State().SystemUnfiledProject().ID]
	if project.Tasks[0].Title != "Already exchanged" {
		t.Fatalf("gs left stale view data: %q", project.Tasks[0].Title)
	}
	if !strings.Contains(model.feedback.message, "0 imported") {
		t.Fatalf("refresh should not need another import: %q", model.feedback.message)
	}
}

func TestManualSyncJoinsAutomaticPassAndDefersAdoption(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	retitleOnDisk(t, pair.laptop, "Incoming while editing")
	exchangeOnce(t, pair.laptop)
	inflight := model.startAutomaticSync()
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	if !model.automaticSync.running || model.feedback.message != "Sync already in progress…" {
		t.Fatalf("manual request did not join the active pass: %q", model.feedback.message)
	}
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	runSession2AppCommand(t, model, inflight)
	projectID := pair.desk.State().SystemUnfiledProject().ID
	if !model.externalReload.owed || model.domain.projects[projectID].Tasks[0].Title != "Shared task" {
		t.Fatal("exchange replaced view data under an active child view")
	}
	if !strings.Contains(model.feedback.message, "view refresh pending") {
		t.Fatalf("missing deferred-refresh feedback: %q", model.feedback.message)
	}
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	runSession2AppCommand(t, model, model.adoptExternalWorkspaceChange())
	if model.domain.projects[projectID].Tasks[0].Title != "Incoming while editing" {
		t.Fatal("deferred incoming change was lost")
	}
}

func TestManualSyncReportsFailureAndCanRetry(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	exchange := model.automaticSync.exchange
	model.automaticSync.exchange = func() (*syncexchange.FolderReport, error) {
		return nil, errors.New("exchange folder offline")
	}
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	if !strings.Contains(model.feedback.message, "Sync failed: exchange folder offline") {
		t.Fatalf("failed pass was hidden: %q", model.feedback.message)
	}
	model.automaticSync.exchange = exchange
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	if model.automaticSync.err != nil || !strings.Contains(model.feedback.message, "Sync: 0 published, 0 imported") {
		t.Fatalf("retry did not recover: %q", model.feedback.message)
	}
}

func TestManualSyncOnUnenrolledWorkspaceExplainsSetup(t *testing.T) {
	model := typedTestApp(t)
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	if model.automaticSync.running || model.automaticSync.manual || !strings.Contains(model.feedback.message, "not enrolled") {
		t.Fatalf("plain workspace did not explain sync setup: %q", model.feedback.message)
	}
}

func TestManualSyncReportsPartialExchange(t *testing.T) {
	model := typedTestApp(t)
	model.automaticSync.exchange = func() (*syncexchange.FolderReport, error) {
		// A transport pass can succeed while individual publications fail or
		// incoming files are waiting for download. Do not call that "up to date".
		return &syncexchange.FolderReport{
			Succeeded: true, Published: []string{"published"}, Imported: 2,
			OutstandingUp: 1, OutstandingDown: 3,
			Unavailable: []string{"cloud-placeholder"}, Errors: []string{"cannot publish blob"},
		}, nil
	}
	runSession2AppCommand(t, model, pressFilterChord(t, model, "gs"))
	for _, want := range []string{
		"1 published, 2 imported", "1 waiting to publish", "3 waiting to apply",
		"files not downloaded yet", "exchange errors (docket sync doctor)",
	} {
		if !strings.Contains(model.feedback.message, want) {
			t.Fatalf("partial exchange omits %q: %q", want, model.feedback.message)
		}
	}
	if model.feedback.severity != flashWarning {
		t.Fatal("partial exchange errors did not get warning feedback")
	}
}

func TestManualSyncConflictPartAgreesWithCount(t *testing.T) {
	for count, want := range map[int]string{
		1: "1 unresolved sync conflict (gc to review)",
		2: "2 unresolved sync conflicts (gc to review)",
	} {
		if got := manualSyncConflictPart(count); got != want {
			t.Fatalf("manualSyncConflictPart(%d) = %q, want %q", count, got, want)
		}
	}
}
