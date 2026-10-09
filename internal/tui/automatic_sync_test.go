package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/tui/presentation"
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
	if calls != 1 || model.automaticSync.running {
		t.Fatal("failed pass did not release its running flag")
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
