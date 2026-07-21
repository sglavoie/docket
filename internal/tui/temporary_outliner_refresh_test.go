package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func openRichTemporaryOutliner(t *testing.T) (*UITestDriver, *OutlinerModel) {
	t.Helper()
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	for _, action := range []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+g"},
		{Type: UITestActionKey, Key: "3"},
		{Type: UITestActionWait},
	} {
		if _, actionErr := driver.ApplyAction(action, ObserveOptions{}); actionErr != nil {
			t.Fatal(actionErr)
		}
	}

	// TemporaryOutliner is still a supported compatibility surface, but the
	// consolidated Task Workspace intentionally has no hidden `go` drill-down
	// alias. Open it through its message contract so this regression continues
	// to test registry-owned refresh behavior rather than retired navigation.
	var targetProject *models.Project
	for _, item := range driver.App().taskWorkspace.Snapshot().Items {
		if item.Project != nil && driver.App().currentProject != nil && item.Project.ID == driver.App().currentProject.ID {
			targetProject = item.Project
			break
		}
	}
	if targetProject == nil || len(targetProject.Tasks) == 0 {
		t.Fatal("fixture did not load a temporary-outliner target project")
	}
	driver.dispatch(ShowOutlinerMsg{Project: targetProject, OpenAtTask: targetProject.Tasks[0], Temporary: true})
	driver.drain()

	if driver.App().currentView() != ViewTemporaryOutliner {
		t.Fatalf("current view = %s, want TemporaryOutliner", driver.App().currentView())
	}
	temporary, ok := driver.App().views.Get(ViewTemporaryOutliner).(*OutlinerModel)
	if !ok || temporary.project == nil {
		t.Fatalf("temporary outliner = %T, want project-backed *OutlinerModel", driver.App().views.Get(ViewTemporaryOutliner))
	}
	if temporary.project == driver.App().currentProject {
		t.Fatal("setup requires temporary outliner and current project to use distinct project pointers")
	}
	if temporary.project.ID != driver.App().currentProject.ID {
		t.Fatalf("temporary project ID = %q, current project ID = %q", temporary.project.ID, driver.App().currentProject.ID)
	}
	return driver, temporary
}

func outlinerContainsTask(outliner *OutlinerModel, taskID string) bool {
	for _, row := range outliner.rows {
		if row.task != nil && row.task.ID == taskID {
			return true
		}
	}
	return false
}

// Regression: a project drill-down from the cross-project outliner owns a
// separate project pointer and lives only in the view registry. Quick Capture
// previously reloaded m.currentProject/m.outliner, leaving this view's Inbox
// count and rows stale until the user navigated away and back.
func TestQuickCaptureRefreshesTemporaryOutlinerImmediately(t *testing.T) {
	driver, temporary := openRichTemporaryOutliner(t)
	beforeCount := len(temporary.project.Tasks)
	temporary.collapsedSections[models.SectionInbox] = true
	temporary.rebuildRows()

	const title = "Temporary outliner refresh probe"
	for _, action := range []UITestAction{
		{Type: UITestActionKey, Key: "i"},
		{Type: UITestActionText, Text: title},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	} {
		if _, actionErr := driver.ApplyAction(action, ObserveOptions{}); actionErr != nil {
			t.Fatal(actionErr)
		}
	}

	if driver.App().currentView() != ViewTemporaryOutliner {
		t.Fatalf("current view = %s, want TemporaryOutliner", driver.App().currentView())
	}
	refreshed := driver.App().views.Get(ViewTemporaryOutliner).(*OutlinerModel)
	if len(refreshed.project.Tasks) != beforeCount+1 {
		t.Fatalf("temporary project task count = %d, want %d", len(refreshed.project.Tasks), beforeCount+1)
	}
	created := refreshed.getCursorTask()
	if created == nil || created.Title != title {
		t.Fatalf("selected task = %v, want %q", created, title)
	}
	if models.SectionForState(created.State()) != models.SectionInbox {
		t.Fatalf("created task section = %q, want Inbox", models.SectionForState(created.State()))
	}
	if refreshed.collapsedSections[models.SectionInbox] {
		t.Fatal("Inbox remained collapsed after navigating to the created task")
	}
	if !outlinerContainsTask(refreshed, created.ID) {
		t.Fatalf("temporary outliner rows do not contain created task %q", title)
	}
	if !strings.Contains(driver.Render(), title) {
		t.Fatalf("rendered temporary outliner does not contain %q", title)
	}
}

// The shared coordinator result path must refresh the same registry-owned
// project pointer, not just the AppModel's canonical project cache.
func TestTransactionResultRefreshesTemporaryOutlinerProjectCopy(t *testing.T) {
	driver, temporary := openRichTemporaryOutliner(t)

	const title = "Post-save temporary refresh probe"
	cmd := driver.App().features.Capture.Capture(app.CaptureRequest{
		ProjectID: temporary.project.ID,
		Title:     title,
		Section:   models.SectionInbox,
	})
	raw := cmd()
	msg, ok := raw.(TransactionResultMsg)
	if !ok {
		t.Fatalf("capture result = %T, want TransactionResultMsg", raw)
	}
	if temporary.project.GetTaskByID(msg.Result.TaskID) != nil {
		t.Fatal("setup requires the temporary project pointer to be stale before shared refresh")
	}

	_ = driver.App().handleTransactionResult(msg)

	created := temporary.project.GetTaskByID(msg.Result.TaskID)
	if created == nil || created.Title != title {
		t.Fatalf("temporary project task = %v, want %q", created, title)
	}
	if !outlinerContainsTask(temporary, msg.Result.TaskID) {
		t.Fatalf("temporary outliner rows do not contain refreshed task %q", title)
	}
}
