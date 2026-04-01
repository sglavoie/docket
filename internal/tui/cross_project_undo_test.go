package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestCrossProjectUndoOrchestration(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	alphaDir := t.TempDir()
	betaDir := t.TempDir()

	const (
		alphaEpicID       = "alpha-epic-001"
		alphaStoryID      = "alpha-story-001"
		alphaLeafID       = "alpha-task-001"
		alphaStandaloneID = "alpha-task-002"
	)

	alphaEpic, _ := models.NewTask(alphaEpicID, 1, models.TaskTypeEpic, "Epic: Authentication")
	alphaStory, _ := models.NewTask(alphaStoryID, 2, models.TaskTypeStory, "Story: Login Flow")
	alphaStory.ParentID = strPtr(alphaEpicID)
	alphaLeaf, _ := models.NewTask(alphaLeafID, 3, models.TaskTypeTask, "Implement login endpoint")
	alphaLeaf.ParentID = strPtr(alphaStoryID)
	alphaStandalone, _ := models.NewTask(alphaStandaloneID, 4, models.TaskTypeTask, "Fix typo in readme")

	alpha := &models.Project{ID: "alpha-proj", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaEpic, alphaStory, alphaLeaf, alphaStandalone}
	alpha.BuildTaskTree()
	if err := storage.SaveProject(alpha); err != nil {
		t.Fatalf("save alpha: %v", err)
	}

	const (
		betaParentID     = "beta-task-001"
		betaChild1ID     = "beta-task-002"
		betaChild2ID     = "beta-task-003"
		betaStandaloneID = "beta-task-004"
	)

	betaParent, _ := models.NewTask(betaParentID, 1, models.TaskTypeTask, "Setup CI pipeline")
	betaChild1, _ := models.NewTask(betaChild1ID, 2, models.TaskTypeTask, "Configure lint")
	betaChild1.ParentID = strPtr(betaParentID)
	betaChild2, _ := models.NewTask(betaChild2ID, 3, models.TaskTypeTask, "Configure tests")
	betaChild2.ParentID = strPtr(betaParentID)
	betaStandalone, _ := models.NewTask(betaStandaloneID, 4, models.TaskTypeTask, "Write API docs")

	beta := &models.Project{ID: "beta-proj", Name: "Beta", Path: betaDir}
	beta.Tasks = []*models.Task{betaParent, betaChild1, betaChild2, betaStandalone}
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

	originalAlpha := readCrossTestFile(t, filepath.Join(alphaDir, "tasks.md"))
	originalBeta := readCrossTestFile(t, filepath.Join(betaDir, "tasks.md"))

	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    []*models.Project{alpha, beta},
	}

	selectedAlpha := alpha.GetTaskByID(alphaLeafID)
	selectedBeta := beta.GetTaskByID(betaChild1ID)
	if selectedAlpha == nil || selectedBeta == nil {
		t.Fatal("could not find selected tasks in loaded projects after reload")
	}
	bulkTasks := []*models.Task{selectedAlpha, selectedBeta}
	m.bulkOp.SetOperation(models.StatusInProgress, bulkTasks, 0, nil)

	resultModel, _ := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)

	alphaBackupPath := storage.GetBackupFilePath(alpha)
	betaBackupPath := storage.GetBackupFilePath(beta)

	if got := readCrossTestFile(t, alphaBackupPath); got != originalAlpha {
		t.Error("Alpha backup should contain pre-mutation tasks.md content")
	}
	if got := readCrossTestFile(t, betaBackupPath); got != originalBeta {
		t.Error("Beta backup should contain pre-mutation tasks.md content")
	}

	if got := readCrossTestFile(t, filepath.Join(alphaDir, "tasks.md")); got == originalAlpha {
		t.Error("Alpha tasks.md should be mutated after bulk status change")
	}
	if got := readCrossTestFile(t, filepath.Join(betaDir, "tasks.md")); got == originalBeta {
		t.Error("Beta tasks.md should be mutated after bulk status change")
	}

	wantMutMsg := fmt.Sprintf("Changed %d tasks to %s", len(bulkTasks), models.StatusInProgress)
	if m.statusMessage != wantMutMsg {
		t.Errorf("status after mutation: want %q, got %q", wantMutMsg, m.statusMessage)
	}
	if got := len(m.crossUndoCtx.ProjectIDs); got != 2 {
		t.Errorf("undo context: want 2 project IDs, got %d", got)
	}

	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false in cross-project mode")
	}
	m = undoResult.(*OutlinerModel)

	undoMsg := m.statusMessage
	if !strings.HasPrefix(undoMsg, "Undo: restored ") {
		t.Errorf("undo status should start with 'Undo: restored ', got %q", undoMsg)
	}
	if !strings.Contains(undoMsg, "Alpha") {
		t.Errorf("undo status should contain project name 'Alpha', got %q", undoMsg)
	}
	if !strings.Contains(undoMsg, "Beta") {
		t.Errorf("undo status should contain project name 'Beta', got %q", undoMsg)
	}
	if strings.Contains(undoMsg, "FAILED") {
		t.Errorf("undo status should not contain 'FAILED', got %q", undoMsg)
	}

	if got := readCrossTestFile(t, filepath.Join(alphaDir, "tasks.md")); got != originalAlpha {
		t.Error("Alpha tasks.md should be restored to pre-mutation content after undo")
	}
	if got := readCrossTestFile(t, filepath.Join(betaDir, "tasks.md")); got != originalBeta {
		t.Error("Beta tasks.md should be restored to pre-mutation content after undo")
	}
	if got := len(m.crossUndoCtx.ProjectIDs); got != 0 {
		t.Errorf("undo context should be cleared after undo, got %d IDs", got)
	}
}