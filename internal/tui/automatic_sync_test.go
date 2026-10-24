package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestAutomaticSyncExchangesAndDefersViewAdoption(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	if model.automaticSync.running {
		t.Fatal("launch exchange did not finish")
	}
	if _, err := pair.desk.Service().Capture(app.CaptureRequest{
		ProjectID: pair.desk.State().SystemUnfiledProject().ID, Title: "Written on desktop",
	}); err != nil {
		t.Fatal(err)
	}
	_, cmd := model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	exchangeOnce(t, pair.laptop)
	assertSyncTaskTitle(t, pair.laptop, "Written on desktop")

	retitleOnDisk(t, pair.laptop, "Written on mobile")
	exchangeOnce(t, pair.laptop)
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	_, cmd = model.Update(tea.FocusMsg{})
	runSession2AppCommand(t, model, cmd)
	assertSyncTaskTitle(t, pair.desk, "Written on mobile")
	if !model.externalReload.owed {
		t.Fatal("incoming changes were not retained for later view adoption")
	}
	project := model.domain.projects[pair.desk.State().SystemUnfiledProject().ID]
	if project.Tasks[0].Title != "Shared task" {
		t.Fatal("background import replaced the task population under the open view")
	}
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	_, cmd = model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	if model.externalReload.owed {
		t.Fatal("returning to the Outliner did not adopt the incoming change")
	}
	project = model.domain.projects[pair.desk.State().SystemUnfiledProject().ID]
	if project.Tasks[0].Title != "Written on mobile" {
		t.Fatalf("Outliner still holds %q", project.Tasks[0].Title)
	}
}

func assertSyncTaskTitle(t *testing.T, rt *engine.Runtime, title string) {
	t.Helper()
	project := rt.State().SystemUnfiledProject()
	_, err := storage.LoadProjectWithDocument(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range project.Tasks {
		if task.Title == title {
			return
		}
	}
	t.Fatalf("device does not hold %q", title)
}

func TestAutomaticSyncRetriesAfterFolderReturns(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	syncStatus, err := pair.desk.SyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	folder := syncStatus.Enrollment.ExchangeFolder
	if err := os.Rename(folder, folder+"-offline"); err != nil {
		t.Fatal(err)
	}
	_, cmd := model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	if model.automaticSync.err == nil || !strings.Contains(model.feedback.syncAdvisoryLine(), "failed") {
		t.Fatal("unreachable exchange was not reported")
	}
	if _, err := pair.desk.Service().Capture(app.CaptureRequest{
		ProjectID: pair.desk.State().SystemUnfiledProject().ID, Title: "Saved offline",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(folder+"-offline", folder); err != nil {
		t.Fatal(err)
	}
	_, cmd = model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	if model.automaticSync.err != nil || strings.Contains(model.feedback.syncAdvisoryLine(), "failed") {
		t.Fatal("successful retry did not clear the failure")
	}
	exchangeOnce(t, pair.laptop)
	assertSyncTaskTitle(t, pair.laptop, "Saved offline")
}

func TestAutomaticSyncCoalescesTriggersAndStopsAtQuit(t *testing.T) {
	model := typedTestApp(t)
	calls := 0
	model.automaticSync.exchange = func() (*syncexchange.FolderReport, error) {
		calls++
		return nil, errors.New("offline")
	}
	first := model.startAutomaticSync()
	if first == nil || model.startAutomaticSync() != nil {
		t.Fatal("overlapping exchange was allowed")
	}
	_, cmd := model.Update(tea.FocusMsg{})
	runSession2AppCommand(t, model, cmd)
	_, cmd = model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	if calls != 0 {
		t.Fatal("focus or timer started a second pass before completion")
	}
	runSession2AppCommand(t, model, first)
	if calls != 2 || model.automaticSync.running || model.automaticSync.pending {
		t.Fatal("focus did not produce exactly one follow-up pass")
	}
	model.lifecycle.quitting = true
	if model.startAutomaticSync() != nil || model.automaticSyncTickCmd() != nil {
		t.Fatal("quitting session scheduled another pass")
	}
}

func TestAutomaticSyncDoesNotEnrollPlainWorkspaces(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.original)
	if err != nil {
		t.Fatal(err)
	}
	if model.automaticSync.exchange != nil || model.startAutomaticSync() != nil || model.automaticSyncTickCmd() != nil {
		t.Fatal("unenrolled workspace scheduled sync")
	}
}

// A local save starts exchange without waiting for a timer or a focus event.
// A second save while the first pass has already published gets its own pass.
func TestAutomaticSyncPublishesSavesDuringAnActivePass(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	capture := func(title string) {
		t.Helper()
		if _, err := pair.desk.Service().Capture(app.CaptureRequest{
			ProjectID: pair.desk.State().SystemUnfiledProject().ID, Title: title,
		}); err != nil {
			t.Fatal(err)
		}
	}
	capture("First save")
	_, cmd := model.Update(status.SaveCompleteMsg{})
	runSession2AppCommand(t, model, cmd)
	exchangeOnce(t, pair.laptop)
	assertSyncTaskTitle(t, pair.laptop, "First save")

	// Finish the worker's I/O, but delay delivering its completion to Update.
	inflight := model.startAutomaticSync()
	finished := inflight()
	capture("Saved after publication")
	_, cmd = model.Update(status.SaveCompleteMsg{})
	runSession2AppCommand(t, model, cmd)
	if !model.automaticSync.pending {
		t.Fatal("save during exchange did not queue a follow-up")
	}
	_, cmd = model.Update(finished)
	runSession2AppCommand(t, model, cmd)
	exchangeOnce(t, pair.laptop)
	assertSyncTaskTitle(t, pair.laptop, "Saved after publication")
	if model.automaticSync.running || model.automaticSync.pending {
		t.Fatal("follow-up did not settle")
	}
}

func TestAutomaticSyncDetectsExternalImportWithoutFocus(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	retitleOnDisk(t, pair.laptop, "Imported elsewhere")
	exchangeOnce(t, pair.laptop)
	exchangeOnce(t, pair.reopenDesk(t))

	model.setInfoStatus("Keep the active feedback")
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	_, cmd := model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	projectID := pair.desk.State().SystemUnfiledProject().ID
	if !model.externalReload.owed || model.domain.projects[projectID].Tasks[0].Title != "Shared task" {
		t.Fatal("polling must remember the change without replacing an active child view")
	}
	setTestPresentationPath(model, presentation.ViewCrossProjectOutliner)
	_, cmd = model.Update(automaticSyncTickMsg{})
	runSession2AppCommand(t, model, cmd)
	if model.externalReload.owed || model.domain.projects[projectID].Tasks[0].Title != "Imported elsewhere" {
		t.Fatal("poll did not adopt the externally imported task")
	}
	if model.feedback.message != "Keep the active feedback" {
		t.Fatalf("poll replaced feedback: %q", model.feedback.message)
	}
	model.pollExternalWorkspaceChanges()
	if model.externalReload.owed {
		t.Fatal("unchanged workspace scheduled another reload")
	}
}

func TestAutomaticSyncTimerDoesNotQueueEndlessPasses(t *testing.T) {
	model := typedTestApp(t)
	calls := 0
	model.automaticSync.exchange = func() (*syncexchange.FolderReport, error) {
		calls++
		return &syncexchange.FolderReport{Succeeded: true}, nil
	}
	first := model.startAutomaticSync()
	for range 3 {
		_, cmd := model.Update(automaticSyncTickMsg{})
		runSession2AppCommand(t, model, cmd)
	}
	runSession2AppCommand(t, model, first)
	if calls != 1 || model.automaticSync.pending {
		t.Fatal("timer queued an unnecessary follow-up")
	}
	model.lifecycle.quitting = true
	if model.requestAutomaticSync() != nil || model.automaticSync.pending {
		t.Fatal("quit scheduled sync")
	}
}

func TestAutomaticSyncRetriesViewRefreshAfterReadFailure(t *testing.T) {
	pair := newEnrolledPair(t)
	model, err := NewAppModel(pair.desk)
	if err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.Init())
	path := filepath.Join(pair.desk.DataDir(), ".tt", "workspace.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("temporarily incomplete"), 0o600); err != nil {
		t.Fatal(err)
	}
	model.pollExternalWorkspaceChanges()
	runSession2AppCommand(t, model, model.adoptExternalWorkspaceChange())
	if !model.externalReload.owed {
		t.Fatal("failed refresh was forgotten")
	}
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	runSession2AppCommand(t, model, model.adoptExternalWorkspaceChange())
	if model.externalReload.owed {
		t.Fatal("refresh did not recover")
	}
}
