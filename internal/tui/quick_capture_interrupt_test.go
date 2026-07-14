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

	outliner := driver.App().outliner
	outliner.cursor = nthTaskRowIndex(t, outliner, 1)
	before := driver.Observe(ObserveOptions{})
	beforeStack := slices.Clone(before.ViewStack)
	// Quick capture is now a stack view pushed over the outliner, so the outliner
	// is no longer the active view; read its cursor directly instead of via the
	// active-view snapshot.
	beforeCursor := outliner.cursor

	driveUITestKeys(t, driver, ";", "i", "enter")
	assertQuickCaptureOpenOver(t, driver, ViewOutliner, beforeStack)
	if got := driver.App().outliner.cursor; got != beforeCursor {
		t.Fatalf("outliner cursor while capture is open = %d, want %d", got, beforeCursor)
	}

	// Exercise the syntax viewport state added by the redesign: expand the guide
	// and scroll it by line and page. None of this may disturb the parked
	// outliner beneath capture (cursor or view stack).
	driveUITestKeys(t, driver, "?", "ctrl+j", "pgdown")
	assertQuickCaptureOpenOver(t, driver, ViewOutliner, beforeStack)
	if got := driver.App().outliner.cursor; got != beforeCursor {
		t.Fatalf("outliner cursor after syntax scroll = %d, want %d", got, beforeCursor)
	}
	// Close the guide so the single-Esc dismissal path below is unchanged.
	driveUITestKeys(t, driver, "?")

	driveUITestKeys(t, driver, "esc")
	after := driver.Observe(ObserveOptions{})
	if after.CurrentView != ViewOutliner.String() {
		t.Fatalf("current view after capture dismiss = %q, want %q", after.CurrentView, ViewOutliner.String())
	}
	if !slices.Equal(after.ViewStack, beforeStack) {
		t.Fatalf("view stack after capture dismiss = %v, want %v", after.ViewStack, beforeStack)
	}
	if got := driver.App().outliner.cursor; got != beforeCursor {
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

	driveUITestKeys(t, driver, "j", "j")
	detail := currentTaskDetail(t, driver)
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

	// Exercise the syntax viewport state: expand the guide and scroll it by line
	// and page. The parked task detail's scroll offset and task identity must be
	// untouched by capture's own viewport activity.
	driveUITestKeys(t, driver, "?", "ctrl+j", "pgdown")
	detail = currentTaskDetail(t, driver)
	if detail.scrollOffset != beforeScroll {
		t.Fatalf("task detail scroll offset after syntax scroll = %d, want %d", detail.scrollOffset, beforeScroll)
	}
	if detail.task == nil || detail.task.ID != beforeTaskID {
		t.Fatalf("task detail task after syntax scroll = %v, want task ID %q", detail.task, beforeTaskID)
	}
	// Close the guide so the single-Esc dismissal path below is unchanged.
	driveUITestKeys(t, driver, "?")

	driveUITestKeys(t, driver, "esc")
	after := driver.Observe(ObserveOptions{})
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
	projects := driver.App().activeProjects()
	if len(projects) == 0 {
		t.Fatal("rich fixture should include a project")
	}
	return driver, ws, projects[0]
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

func assertQuickCaptureOpenOver(t *testing.T, driver *UITestDriver, view ViewType, priorStack []string) {
	t.Helper()
	obs := driver.Observe(ObserveOptions{})
	// Quick capture is now a normal stack view pushed on top of the prior view.
	if obs.CurrentView != ViewQuickCapture.String() {
		t.Fatalf("current view while capture is open = %q, want %q", obs.CurrentView, ViewQuickCapture.String())
	}
	wantStack := append(slices.Clone(priorStack), ViewQuickCapture.String())
	if !slices.Equal(obs.ViewStack, wantStack) {
		t.Fatalf("view stack while capture is open = %v, want %v", obs.ViewStack, wantStack)
	}
	if len(priorStack) == 0 || priorStack[len(priorStack)-1] != view.String() {
		t.Fatalf("capture should sit over %q; prior stack = %v", view.String(), priorStack)
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
