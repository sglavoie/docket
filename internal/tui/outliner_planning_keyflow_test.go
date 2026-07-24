package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// installCrossOutlinerRoot rebuilds a ledger-backed AppModel so the cross-project
// Outliner is the live sole-root view with a single on-disk task under the
// cursor. It returns the owning project so keyflow assertions can read committed
// state straight from disk.
func installCrossOutlinerRoot(t *testing.T, m *AppModel, task *models.Task) *models.Project {
	t.Helper()
	project := m.currentProject
	cross := NewOutlinerModel(nil, m.configManager, m.runtime)
	cross.queueIDs = m.focusQueueIDs
	cross.dailyPlanIDs = m.dailyPlanIDs
	cross.loadedProjects = []*models.Project{project}
	cross.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
	cross.cursor = 0
	cross.SetSize(m.width, m.height-2)
	m.crossOutliner = cross
	m.views.Register(ViewCrossProjectOutliner, cross)
	m.viewStack = []PresentationID{ViewCrossProjectOutliner}
	return project
}

// TestOutlinerRootPlanningAndReviewKeys locks FR-001: from the sole Outliner
// root, q toggles Next membership and w toggles Today membership through the
// durable planning transaction path, and R marks/resets the persisted daily
// review timestamp in Daily Review mode while staying non-conflicting outside it.
// Every case drives a real KeyPressMsg through the shell so the routing — not
// just the underlying handlers — is exercised end to end.
func TestOutlinerRootPlanningAndReviewKeys(t *testing.T) {
	t.Run("q promotes the cursor task to Next through the durable ledger", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		project := installCrossOutlinerRoot(t, m, task)

		m.Update(syntheticKey('q'))

		if st := diskTaskState(t, project, task.ID); st != models.StateNext {
			t.Fatalf("state after q = %s, want next", st)
		}
		if got := diskFocusQueueLen(t, dataDir); got != 1 {
			t.Fatalf("Next order after q = %d, want 1 (appended in the same txn)", got)
		}
	})

	t.Run("q removes a Next task, dropping its order entry in the same txn", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateNext)
		m.seedDiskFocusQueue(t, task.ID, m.currentProject.ID)
		project := installCrossOutlinerRoot(t, m, task)

		m.Update(syntheticKey('q'))

		if st := diskTaskState(t, project, task.ID); st != models.StateReady {
			t.Fatalf("state after q = %s, want ready", st)
		}
		if got := diskFocusQueueLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after q = %d, want 0 (dropped by reconciliation)", got)
		}
	})

	t.Run("w adds the cursor task to Today through the durable planning path", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		installCrossOutlinerRoot(t, m, task)

		m.Update(syntheticKey('w'))

		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after w = %d, want 1", got)
		}
	})

	t.Run("R marks then resets the persisted review timestamp in Daily Review mode", func(t *testing.T) {
		m, _, task, _ := newPlanningLedgerApp(t, models.StateReady)
		installCrossOutlinerRoot(t, m, task)
		m.crossOutliner.reviewMode = reviewDaily

		_, cmd := m.crossOutliner.handleKeyMsg(syntheticKey('R'))
		if cmd == nil {
			t.Fatal("R in Daily Review mode returned no command")
		}
		if msg := cmd(); !isType[MarkDailyReviewDoneMsg](msg) {
			t.Fatalf("R (unmarked) emitted %T, want MarkDailyReviewDoneMsg", msg)
		} else {
			m.Update(msg)
		}
		if m.runtime.Planning().GetLastDailyReviewAt() == nil {
			t.Fatal("review timestamp not persisted after first R")
		}

		_, cmd = m.crossOutliner.handleKeyMsg(syntheticKey('R'))
		if cmd == nil {
			t.Fatal("second R in Daily Review mode returned no command")
		}
		if msg := cmd(); !isType[ResetDailyReviewMsg](msg) {
			t.Fatalf("R (marked) emitted %T, want ResetDailyReviewMsg", msg)
		} else {
			m.Update(msg)
		}
		if m.runtime.Planning().GetLastDailyReviewAt() != nil {
			t.Fatal("review timestamp not cleared after second R")
		}
	})

	t.Run("R outside Daily Review mode leaves the review timestamp untouched", func(t *testing.T) {
		m, _, task, _ := newPlanningLedgerApp(t, models.StateReady)
		installCrossOutlinerRoot(t, m, task)
		m.crossOutliner.reviewMode = reviewNone

		if before := m.runtime.Planning().GetLastDailyReviewAt(); before != nil {
			t.Fatalf("precondition review timestamp = %v, want nil", before)
		}

		m.crossOutliner.handleKeyMsg(syntheticKey('R'))

		if after := m.runtime.Planning().GetLastDailyReviewAt(); after != nil {
			t.Fatalf("R outside Daily Review set review timestamp to %v, want nil (reload branch)", after)
		}
	})
}

// TestPlanningOnlyTodayRowsRefreshThroughGlobalKeys locks the live-view half of
// the w/undo/redo contract. Every mutation travels through AppModel.Update, then
// the assertions render the already-loaded cross-project Outliner without a
// project reload. This catches stale rows hidden by the cross-project
// fingerprint guard rather than merely proving planning.json changed.
func TestPlanningOnlyTodayRowsRefreshThroughGlobalKeys(t *testing.T) {
	ctrlR := syntheticCtrlKey('r')

	t.Run("All add updates Today count and indicator through undo and redo", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		project := installCrossOutlinerRoot(t, m, task)
		cross := m.crossOutliner
		cross.selectOutlinerMode(ModeAll)
		if !cross.moveCursorToProjectTask(task.ID, project.ID) {
			t.Fatal("task missing from All before planning toggle")
		}
		_ = cross.View().Content // establish the memoized pre-toggle rows

		sendAppKey(m, syntheticKey('w'))
		assertPlanningOnlyTodayVisualState(t, m, task, project, 1, true)
		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after w = %d, want 1", got)
		}

		m = sendAppKey(m, syntheticKey('u'))
		assertPlanningOnlyTodayVisualState(t, m, task, project, 0, false)
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after undo = %d, want 0", got)
		}

		m = sendAppKey(m, ctrlR)
		assertPlanningOnlyTodayVisualState(t, m, task, project, 1, true)
		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after redo = %d, want 1", got)
		}
	})

	t.Run("Backlog add disappears then undo and redo rebuild rows", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		project := installCrossOutlinerRoot(t, m, task)
		cross := m.crossOutliner
		cross.selectOutlinerMode(ModeBacklog)
		if !cross.moveCursorToProjectTask(task.ID, project.ID) {
			t.Fatal("task missing from Backlog before planning toggle")
		}
		_ = cross.View().Content // establish the memoized pre-toggle rows

		m = sendAppKey(m, syntheticKey('w'))
		assertPlanningOnlyTodayVisualState(t, m, task, project, 1, true)
		assertPlanningTaskRow(t, cross, task.ID, false)
		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after Backlog w = %d, want 1", got)
		}

		m = sendAppKey(m, syntheticKey('u'))
		assertPlanningOnlyTodayVisualState(t, m, task, project, 0, false)
		assertPlanningTaskRow(t, cross, task.ID, true)

		m = sendAppKey(m, ctrlR)
		assertPlanningOnlyTodayVisualState(t, m, task, project, 1, true)
		assertPlanningTaskRow(t, cross, task.ID, false)
	})

	t.Run("scoped Today removal drops selected row and clamps cursor", func(t *testing.T) {
		m, _, first, dataDir := newPlanningLedgerApp(t, models.StateReady)
		project := m.currentProject
		second := seedTask(t, 2, "Second planned task", models.StateReady)
		project.Tasks = append(project.Tasks, second)
		project.BuildTaskTree()
		seedPlanningOnlyDailyPlan(t, m, first, second)

		installCrossOutlinerRoot(t, m, first)
		cross := m.crossOutliner
		cross.pushProjectScope(project.ID)
		cross.selectOutlinerMode(ModeToday)
		if !cross.moveCursorToProjectTask(second.ID, project.ID) {
			t.Fatalf("second task missing from scoped Today rows: %v", planningTaskRowIDs(cross))
		}
		_ = cross.View().Content // establish the memoized pre-toggle rows

		sendAppKey(m, syntheticKey('w'))
		rendered := ansi.Strip(cross.View().Content)
		if strings.Contains(rendered, second.Title) {
			t.Fatalf("removed Today task still rendered in scoped view: %q", rendered)
		}
		assertPlanningTaskRow(t, cross, first.ID, true)
		assertPlanningTaskRow(t, cross, second.ID, false)
		if cross.cursor != 0 {
			t.Fatalf("cursor after removing last scoped Today row = %d, want 0", cross.cursor)
		}
		if cross.scopedProjectID != project.ID {
			t.Fatalf("scope after Today removal = %q, want %q", cross.scopedProjectID, project.ID)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 1 {
			t.Fatalf("Daily Plan after scoped Today removal = %d, want 1", got)
		}
	})

	t.Run("failed persistence rolls back live membership without forcing refresh", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		project := installCrossOutlinerRoot(t, m, task)
		cross := m.crossOutliner
		cross.selectOutlinerMode(ModeAll)
		if !cross.moveCursorToProjectTask(task.ID, project.ID) {
			t.Fatal("task missing from All before failed planning toggle")
		}
		_ = cross.View().Content // consume the mode-transition rebuild
		cross.crossForceRebuild = false

		blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o600); err != nil {
			t.Fatalf("create blocked workspace root: %v", err)
		}
		domains := config.NewDomains()
		domains.Catalog.UpsertProject(project)
		blockedRuntime := config.NewRuntimeState(blockedRoot, domains)
		m.runtime = blockedRuntime
		cross.runtime = blockedRuntime
		m.statusMessage = "unchanged"

		m = sendAppKey(m, syntheticKey('w'))

		if m.err == nil {
			t.Fatal("failed planning commit should surface an error")
		}
		identity := taskIdentityFor(task, project)
		if m.dailyPlanIDs[identity] || len(m.dailyPlan) != 0 {
			t.Fatalf("failed planning commit leaked live membership: slice=%v ids=%v", m.dailyPlan, m.dailyPlanIDs)
		}
		if m.lastToggleUndoKind != toggleUndoNone || m.lastDailyPlanToggle != nil {
			t.Fatal("failed planning commit recorded an undo entry")
		}
		if cross.crossForceRebuild {
			t.Fatal("failed planning commit requested a successful row rebuild")
		}
		if got := strings.TrimSpace(ansi.Strip(cross.renderTaskQueueIndicator(task, project))); got != "" {
			t.Fatalf("failed planning commit rendered Today indicator %q", got)
		}
		if got := cross.workModeItemCount(ModeToday); got != 0 {
			t.Fatalf("failed planning commit rendered Today count %d, want 0", got)
		}
		if m.statusMessage != "unchanged" {
			t.Fatalf("failed planning commit changed success status to %q", m.statusMessage)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("durable Daily Plan after failed commit = %d, want 0", got)
		}
	})
}

func seedPlanningOnlyDailyPlan(t *testing.T, m *AppModel, tasks ...*models.Task) {
	t.Helper()
	items := make([]config.DailyPlanItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, config.DailyPlanItem{
			TaskID:      task.ID,
			ProjectID:   m.currentProject.ID,
			PlannedDate: "2026-07-24",
		})
	}
	if err := commitPlanningToRuntime(m.runtime, func(p *config.PlanningState) {
		p.SetDailyPlan(items)
	}); err != nil {
		t.Fatalf("seed Daily Plan: %v", err)
	}
	m.rebuildPlanningFromRuntime()
}

func assertPlanningOnlyTodayVisualState(
	t *testing.T,
	m *AppModel,
	task *models.Task,
	project *models.Project,
	wantCount int,
	wantIndicator bool,
) {
	t.Helper()
	cross := m.crossOutliner
	rendered := ansi.Strip(cross.View().Content)
	label := fmt.Sprintf("Today %d", wantCount)
	if !strings.Contains(rendered, label) {
		t.Fatalf("rendered work strip missing %q: %q", label, rendered)
	}
	if got := cross.workModeItemCount(ModeToday); got != wantCount {
		t.Fatalf("Today item count = %d, want %d", got, wantCount)
	}
	identity := taskIdentityFor(task, project)
	if got := m.dailyPlanIDs[identity]; got != wantIndicator {
		t.Fatalf("live Today membership = %v, want %v", got, wantIndicator)
	}
	indicator := strings.TrimSpace(ansi.Strip(cross.renderTaskQueueIndicator(task, project)))
	if wantIndicator && indicator != "w" {
		t.Fatalf("rendered Today indicator = %q, want w", indicator)
	}
	if !wantIndicator && indicator != "" {
		t.Fatalf("rendered Today indicator = %q, want empty", indicator)
	}
}

func assertPlanningTaskRow(t *testing.T, cross *OutlinerModel, taskID string, want bool) {
	t.Helper()
	for _, id := range planningTaskRowIDs(cross) {
		if id == taskID {
			if !want {
				t.Fatalf("task %s unexpectedly remained in rendered rows %v", taskID, planningTaskRowIDs(cross))
			}
			return
		}
	}
	if want {
		t.Fatalf("task %s missing from rendered rows %v", taskID, planningTaskRowIDs(cross))
	}
}

func planningTaskRowIDs(cross *OutlinerModel) []string {
	var ids []string
	for _, row := range cross.rows {
		if row.isTaskRow() && row.task != nil {
			ids = append(ids, row.task.ID)
		}
	}
	return ids
}

// isType reports whether msg is exactly of type T.
func isType[T any](msg any) bool {
	_, ok := msg.(T)
	return ok
}
