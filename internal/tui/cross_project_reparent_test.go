package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestCrossProjectReparent verifies that a ParentSelectedMsg received by the
// OutlinerModel in cross-project mode reparents the task in its owning project
// (not some other loaded project), persists the change, and records an undo
// snapshot.
func TestCrossProjectReparent(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	alphaDir := t.TempDir()
	betaDir := t.TempDir()

	const (
		alphaEpicAID = "alpha-epic-a"
		alphaEpicBID = "alpha-epic-b"
		alphaLeafID  = "alpha-leaf"
	)

	alphaEpicA, _ := models.NewTask(alphaEpicAID, 1, models.TaskTypeEpic, "Epic A")
	alphaEpicB, _ := models.NewTask(alphaEpicBID, 2, models.TaskTypeEpic, "Epic B")
	alphaLeaf, _ := models.NewTask(alphaLeafID, 3, models.TaskTypeTask, "Leaf under A")
	alphaLeaf.ParentID = strPtr(alphaEpicAID)

	alpha := &models.Project{ID: "alpha-proj", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaEpicA, alphaEpicB, alphaLeaf}
	alpha.BuildTaskTree()
	if err := storage.SaveProject(alpha); err != nil {
		t.Fatalf("save alpha: %v", err)
	}

	betaEpic, _ := models.NewTask("beta-epic", 1, models.TaskTypeEpic, "Beta Epic")
	beta := &models.Project{ID: "beta-proj", Name: "Beta", Path: betaDir}
	beta.Tasks = []*models.Task{betaEpic}
	beta.BuildTaskTree()
	if err := storage.SaveProject(beta); err != nil {
		t.Fatalf("save beta: %v", err)
	}

	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("load alpha: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("load beta: %v", err)
	}

	depth := 5
	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			ViewBundle:     ViewBundle{config: &config.Config{UndoHistoryDepth: &depth}},
			projectService: NewProjectService(),
		},
		loadedProjects: []*models.Project{alpha, beta},
	}

	newParentID := alphaEpicBID
	_, cmd := m.handleCrossProjectReparent(ParentSelectedMsg{
		ReparentTaskID: alphaLeafID,
		ParentID:       &newParentID,
		Source:         ViewCrossProjectOutliner,
	})
	if cmd == nil {
		t.Fatal("handleCrossProjectReparent returned nil cmd")
	}
	if _, ok := cmd().(ErrorMsg); ok {
		t.Fatalf("save cmd returned ErrorMsg")
	}

	// Reload alpha from disk to verify persistence.
	alphaReload := &models.Project{ID: alpha.ID, Name: alpha.Name, Path: alphaDir}
	if err := storage.LoadProject(alphaReload); err != nil {
		t.Fatalf("reload alpha: %v", err)
	}
	leaf := alphaReload.GetTaskByID(alphaLeafID)
	if leaf == nil {
		t.Fatal("leaf not found after reload")
	}
	if leaf.ParentID == nil || *leaf.ParentID != alphaEpicBID {
		got := "nil"
		if leaf.ParentID != nil {
			got = *leaf.ParentID
		}
		t.Errorf("leaf parent: want %q, got %q", alphaEpicBID, got)
	}

	// Verify undo snapshot was recorded for alpha (and only alpha).
	alphaSnap := filepath.Join(alphaDir, ".undo", "0001.md")
	if _, err := os.Stat(alphaSnap); err != nil {
		t.Errorf("expected undo snapshot at %s: %v", alphaSnap, err)
	}
	betaSnap := filepath.Join(betaDir, ".undo", "0001.md")
	if _, err := os.Stat(betaSnap); err == nil {
		t.Errorf("beta should not have an undo snapshot (only alpha was mutated)")
	}

	// Verify undo context tracks only alpha.
	if got := m.crossUndoCtx.Action; got != "Reparent Task" {
		t.Errorf("crossUndoCtx.Action: want 'Reparent Task', got %q", got)
	}
	if got := m.crossUndoCtx.ProjectIDs; len(got) != 1 || got[0] != alpha.ID {
		t.Errorf("crossUndoCtx.ProjectIDs: want [%q], got %v", alpha.ID, got)
	}
}

// TestCrossProjectReparentToRoot verifies that picking "None" (nil ParentID)
// detaches the task from its current parent in cross-project mode.
func TestCrossProjectReparentToRoot(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	alphaDir := t.TempDir()
	alphaEpic, _ := models.NewTask("epic", 1, models.TaskTypeEpic, "Epic")
	alphaLeaf, _ := models.NewTask("leaf", 2, models.TaskTypeTask, "Leaf")
	alphaLeaf.ParentID = strPtr("epic")
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaEpic, alphaLeaf}
	alpha.BuildTaskTree()
	if err := storage.SaveProject(alpha); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("load: %v", err)
	}

	depth := 5
	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			ViewBundle:     ViewBundle{config: &config.Config{UndoHistoryDepth: &depth}},
			projectService: NewProjectService(),
		},
		loadedProjects: []*models.Project{alpha},
	}

	_, cmd := m.handleCrossProjectReparent(ParentSelectedMsg{
		ReparentTaskID: "leaf",
		ParentID:       nil,
		Source:         ViewCrossProjectOutliner,
	})
	if cmd == nil {
		t.Fatal("handleCrossProjectReparent returned nil cmd")
	}
	if _, ok := cmd().(ErrorMsg); ok {
		t.Fatal("save cmd returned ErrorMsg")
	}

	reload := &models.Project{ID: alpha.ID, Name: alpha.Name, Path: alphaDir}
	if err := storage.LoadProject(reload); err != nil {
		t.Fatalf("reload: %v", err)
	}
	leaf := reload.GetTaskByID("leaf")
	if leaf == nil {
		t.Fatal("leaf missing")
	}
	if leaf.ParentID != nil {
		t.Errorf("leaf ParentID: want nil, got %q", *leaf.ParentID)
	}
}
