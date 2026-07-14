package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// pinnedPopupFixture builds a disk-backed AppModel with two pinned tasks in one
// project, an application service over the same in-memory pointers, and a primed
// pinned popup. The popup's unpin/repin/reorder handlers commit through the
// service (ledger-undoable) instead of a raw SaveWithBackupSync.
func pinnedPopupFixture(t *testing.T) (*AppModel, *models.Project, string, string) {
	t.Helper()

	dataDir := t.TempDir()
	early := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)

	taskA, err := models.NewTask("pin-task-a", 1, models.TaskTypeTask, "First pinned")
	if err != nil {
		t.Fatalf("NewTask a: %v", err)
	}
	taskA.Pinned, taskA.PinnedAt = true, &early
	taskB, err := models.NewTask("pin-task-b", 2, models.TaskTypeTask, "Second pinned")
	if err != nil {
		t.Fatalf("NewTask b: %v", err)
	}
	taskB.Pinned, taskB.PinnedAt = true, &late

	proj := &models.Project{ID: "pin-proj", Name: "Pinned", Path: filepath.Join(dataDir, "projects", "pin-proj")}
	proj.Tasks = []*models.Task{taskA, taskB}
	proj.NextShortID = 3
	proj.BuildTaskTree()
	if saveErr := storage.SaveProject(proj); saveErr != nil {
		t.Fatalf("save proj: %v", saveErr)
	}
	if loadErr := storage.LoadProject(proj); loadErr != nil {
		t.Fatalf("load proj: %v", loadErr)
	}

	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, []*models.Project{proj})

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, proj)
	svc := app.NewService(repo, nil, nil)

	m := &AppModel{
		runtime:    testRuntimeAt(cfg.DataDir(), cfg),
		projects:   map[string]*models.Project{proj.ID: proj},
		persister:  NewProjectService(),
		appService: svc,
		views:      NewViewRegistry(),
	}
	m.pinnedPopupItems = m.collectPinnedTasks()
	m.rebuildPinnedPopupFilter()
	m.pinnedPopupCursor = 0
	m.showPinnedPopup = true

	return m, proj, taskA.ID, taskB.ID
}

func TestPinnedPopupReorderThroughAppService(t *testing.T) {
	m, _, taskAID, taskBID := pinnedPopupFixture(t)

	// Sorted ascending by PinnedAt: [A(early), B(late)]. Moving A down swaps the
	// timestamps so the order becomes [B, A].
	if m.pinnedPopupItems[0].Task.ID != taskAID {
		t.Fatalf("initial top = %s, want A", m.pinnedPopupItems[0].Task.ID)
	}
	if _, cmd := m.reorderPinnedTask(1); cmd != nil {
		t.Fatalf("reorder returned unexpected command: %v", cmd)
	}
	if m.pinnedPopupItems[0].Task.ID != taskBID || m.pinnedPopupItems[1].Task.ID != taskAID {
		t.Fatalf("order after reorder = [%s, %s], want [B, A]", m.pinnedPopupItems[0].Task.ID, m.pinnedPopupItems[1].Task.ID)
	}
	if m.pinnedPopupCursor != 1 {
		t.Fatalf("cursor after reorder = %d, want 1 (follows moved item)", m.pinnedPopupCursor)
	}
}

func TestPinnedPopupUnpinAndRepinThroughAppService(t *testing.T) {
	m, proj, taskAID, _ := pinnedPopupFixture(t)

	// Unpin the top task (A). It leaves the popup and its disk state clears.
	if _, cmd := m.unpinFromPopup(); cmd != nil {
		t.Fatalf("unpin returned unexpected command: %v", cmd)
	}
	if len(m.pinnedPopupItems) != 1 {
		t.Fatalf("pinned count after unpin = %d, want 1", len(m.pinnedPopupItems))
	}
	fresh := &models.Project{ID: proj.ID, Name: proj.Name, Path: proj.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := fresh.GetTaskByID(taskAID); got == nil || got.Pinned {
		t.Fatalf("task A still pinned on disk: %+v", got)
	}
	if m.pinnedLastUnpinned == nil {
		t.Fatal("pinnedLastUnpinned not recorded for repin")
	}

	// Repin restores A.
	if _, cmd := m.repinFromPopup(); cmd != nil {
		t.Fatalf("repin returned unexpected command: %v", cmd)
	}
	if len(m.pinnedPopupItems) != 2 {
		t.Fatalf("pinned count after repin = %d, want 2", len(m.pinnedPopupItems))
	}
	fresh2 := &models.Project{ID: proj.ID, Name: proj.Name, Path: proj.Path}
	if err := storage.LoadProject(fresh2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := fresh2.GetTaskByID(taskAID); got == nil || !got.Pinned {
		t.Fatalf("task A not repinned on disk: %+v", got)
	}
}
