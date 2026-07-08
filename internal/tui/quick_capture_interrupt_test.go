package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestQuickCaptureInterruptPreservesOutlinerContext(t *testing.T) {
	driver, _, project := newQuickCaptureInterruptFixture(t)
	openFixtureOutliner(t, driver, project)

	before := driver.Observe(ObserveOptions{})
	outliner := driver.App().outliner
	outliner.cursor = nthTaskRowIndex(t, outliner, 1)
	before = driver.Observe(ObserveOptions{})
	beforeStack := slices.Clone(before.ViewStack)
	beforeCursor := snapshotInt(t, before.Snapshot, "cursor")

	driveUITestKeys(t, driver, ";", "i", "enter")
	assertQuickCaptureOpenOver(t, driver, ViewOutliner, beforeStack)
	if got := snapshotInt(t, driver.Observe(ObserveOptions{}).Snapshot, "cursor"); got != beforeCursor {
		t.Fatalf("outliner cursor while capture is open = %d, want %d", got, beforeCursor)
	}

	driveUITestKeys(t, driver, "esc")
	after := driver.Observe(ObserveOptions{})
	if driver.App().quickCaptureOverlay != nil {
		t.Fatal("quick capture overlay should close after esc")
	}
	if after.CurrentView != ViewOutliner.String() {
		t.Fatalf("current view after capture dismiss = %q, want %q", after.CurrentView, ViewOutliner.String())
	}
	if !slices.Equal(after.ViewStack, beforeStack) {
		t.Fatalf("view stack after capture dismiss = %v, want %v", after.ViewStack, beforeStack)
	}
	if got := snapshotInt(t, after.Snapshot, "cursor"); got != beforeCursor {
		t.Fatalf("outliner cursor after capture dismiss = %d, want %d", got, beforeCursor)
	}
}

func TestQuickCaptureInterruptPreservesTaskDetailContext(t *testing.T) {
	driver, _, project := newQuickCaptureInterruptFixture(t)
	openFixtureOutliner(t, driver, project)

	_ = driver.Observe(ObserveOptions{})
	outliner := driver.App().outliner
	task := outliner.rows[nthTaskRowIndex(t, outliner, 1)].task
	task.Description = strings.Repeat("Interrupt verification detail line.\n", 40)

	driver.dispatch(ShowTaskDetailMsg{Task: task, Project: project})
	driver.drain()
	detail := currentTaskDetail(t, driver)

	driveUITestKeys(t, driver, "j", "j")
	detail = currentTaskDetail(t, driver)
	if detail.scrollOffset == 0 {
		t.Fatal("task detail scroll offset should be non-zero before opening capture")
	}
	beforeScroll := detail.scrollOffset
	beforeTaskID := detail.task.ID
	beforeStack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

	driveUITestKeys(t, driver, ";", "i", "enter")
	assertQuickCaptureOpenOver(t, driver, ViewTaskDetail, beforeStack)
	detail = currentTaskDetail(t, driver)
	if detail.scrollOffset != beforeScroll {
		t.Fatalf("task detail scroll offset while capture is open = %d, want %d", detail.scrollOffset, beforeScroll)
	}
	if detail.task == nil || detail.task.ID != beforeTaskID {
		t.Fatalf("task detail task while capture is open = %v, want task ID %q", detail.task, beforeTaskID)
	}

	driveUITestKeys(t, driver, "esc")
	after := driver.Observe(ObserveOptions{})
	if driver.App().quickCaptureOverlay != nil {
		t.Fatal("quick capture overlay should close after esc")
	}
	if after.CurrentView != ViewTaskDetail.String() {
		t.Fatalf("current view after capture dismiss = %q, want %q", after.CurrentView, ViewTaskDetail.String())
	}
	if !slices.Equal(after.ViewStack, beforeStack) {
		t.Fatalf("view stack after capture dismiss = %v, want %v", after.ViewStack, beforeStack)
	}
	detail = currentTaskDetail(t, driver)
	if detail.scrollOffset != beforeScroll {
		t.Fatalf("task detail scroll offset after capture dismiss = %d, want %d", detail.scrollOffset, beforeScroll)
	}
	if detail.task == nil || detail.task.ID != beforeTaskID {
		t.Fatalf("task detail task after capture dismiss = %v, want task ID %q", detail.task, beforeTaskID)
	}
}

func newQuickCaptureInterruptFixture(t *testing.T) (*UITestDriver, *uitestWorkspace, *models.Project) {
	t.Helper()
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	if len(ws.Config.Projects) == 0 {
		t.Fatal("rich fixture should include a project")
	}
	return driver, ws, ws.Config.Projects[0]
}

func openFixtureOutliner(t *testing.T, driver *UITestDriver, project *models.Project) {
	t.Helper()
	driver.pendingCmd = driver.App().selectProjectNow(project)
	driver.drain()
	obs := driver.Observe(ObserveOptions{})
	if obs.CurrentView != ViewOutliner.String() {
		t.Fatalf("current view = %q, want %q", obs.CurrentView, ViewOutliner.String())
	}
	if driver.App().outliner == nil {
		t.Fatal("outliner should be registered after selecting fixture project")
	}
}

func nthTaskRowIndex(t *testing.T, outliner *OutlinerModel, n int) int {
	t.Helper()
	if outliner == nil {
		t.Fatal("outliner is nil")
	}
	seen := 0
	for i, row := range outliner.rows {
		if row.task == nil {
			continue
		}
		if seen == n {
			return i
		}
		seen++
	}
	t.Fatalf("outliner has %d task rows, want row index %d", seen, n)
	return 0
}

func driveUITestKeys(t *testing.T, driver *UITestDriver, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, err := driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: key}, ObserveOptions{}); err != nil {
			t.Fatalf("ApplyAction key %q: %v", key, err)
		}
	}
}

func assertQuickCaptureOpenOver(t *testing.T, driver *UITestDriver, view ViewType, wantStack []string) {
	t.Helper()
	app := driver.App()
	if app.quickCaptureOverlay == nil {
		t.Fatal("quick capture overlay should be open")
	}
	obs := driver.Observe(ObserveOptions{})
	if obs.CurrentView != view.String() {
		t.Fatalf("current view while capture is open = %q, want %q", obs.CurrentView, view.String())
	}
	if !slices.Equal(obs.ViewStack, wantStack) {
		t.Fatalf("view stack while capture is open = %v, want %v", obs.ViewStack, wantStack)
	}
}

func currentTaskDetail(t *testing.T, driver *UITestDriver) *TaskDetailModel {
	t.Helper()
	view := driver.App().views.Get(ViewTaskDetail)
	detail, ok := view.(*TaskDetailModel)
	if !ok || detail == nil {
		t.Fatalf("task detail view = %T, want *TaskDetailModel", view)
	}
	return detail
}

func snapshotInt(t *testing.T, snap map[string]any, key string) int {
	t.Helper()
	if snap == nil {
		t.Fatalf("snapshot missing key %q: snapshot is nil", key)
	}
	v, ok := snap[key]
	if !ok {
		t.Fatalf("snapshot missing key %q; got %v", key, snap)
	}
	n, ok := v.(int)
	if !ok {
		t.Fatalf("snapshot key %q = %T, want int", key, v)
	}
	return n
}
