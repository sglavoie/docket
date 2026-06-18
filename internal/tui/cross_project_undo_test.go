package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
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

	betaParent, _ := models.NewTask(betaParentID, 1, models.TaskTypeEpic, "Setup CI pipeline")
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

	undoDepth := 5
	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = &undoDepth
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
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
	alphaUndo, err := storage.LoadUndoHistory(alpha)
	if err != nil {
		t.Fatalf("load alpha undo history: %v", err)
	}
	betaUndo, err := storage.LoadUndoHistory(beta)
	if err != nil {
		t.Fatalf("load beta undo history: %v", err)
	}
	if len(alphaUndo) != 1 || len(betaUndo) != 1 {
		t.Fatalf("expected one undo entry per project, got alpha=%d beta=%d", len(alphaUndo), len(betaUndo))
	}
	if alphaUndo[0].GroupID == "" || alphaUndo[0].GroupID != betaUndo[0].GroupID {
		t.Fatalf("undo group IDs should match and be non-empty, got %q and %q", alphaUndo[0].GroupID, betaUndo[0].GroupID)
	}
	if got := strings.Join(alphaUndo[0].GroupProjectIDs, ","); got != "alpha-proj,beta-proj" {
		t.Fatalf("group project IDs = %q, want alpha-proj,beta-proj", got)
	}

	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false in cross-project mode")
	}
	m = undoResult.(*OutlinerModel)

	undoMsg := m.statusMessage
	if !strings.HasPrefix(undoMsg, "Undid: Bulk Status Change") {
		t.Errorf("undo status should describe persistent undo, got %q", undoMsg)
	}

	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("reload alpha after undo: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("reload beta after undo: %v", err)
	}
	if got := alpha.GetTaskByID(alphaLeafID).Status; got != models.StatusTodo {
		t.Errorf("alpha task status after undo = %s, want %s", got, models.StatusTodo)
	}
	if got := beta.GetTaskByID(betaChild1ID).Status; got != models.StatusTodo {
		t.Errorf("beta task status after undo = %s, want %s", got, models.StatusTodo)
	}
	if got := len(m.crossUndoCtx.ProjectIDs); got != 0 {
		t.Errorf("undo context should be cleared after undo, got %d IDs", got)
	}

	redoResult, _, ok := m.crossHandleRedo("ctrl+r")
	if !ok {
		t.Fatal("crossHandleRedo returned ok=false in cross-project mode")
	}
	m = redoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Redo: Bulk Status Change") {
		t.Errorf("redo status should describe persistent redo, got %q", m.statusMessage)
	}
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("reload alpha after redo: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("reload beta after redo: %v", err)
	}
	if got := alpha.GetTaskByID(alphaLeafID).Status; got != models.StatusInProgress {
		t.Errorf("alpha task status after redo = %s, want %s", got, models.StatusInProgress)
	}
	if got := beta.GetTaskByID(betaChild1ID).Status; got != models.StatusInProgress {
		t.Errorf("beta task status after redo = %s, want %s", got, models.StatusInProgress)
	}
}

func TestCrossProjectUndoRedoReappliesPostRestoreSideEffects(t *testing.T) {
	const (
		doneID   = "done-first"
		activeID = "active-second"
	)

	doneTask, err := models.NewTask(doneID, 1, models.TaskTypeTask, "Completed first")
	if err != nil {
		t.Fatalf("new done task: %v", err)
	}
	doneTask.Complete()
	activeTask, err := models.NewTask(activeID, 2, models.TaskTypeTask, "Active second")
	if err != nil {
		t.Fatalf("new active task: %v", err)
	}

	project := &models.Project{
		ID:          "alpha-proj",
		Name:        "Alpha",
		Path:        t.TempDir(),
		Tasks:       []*models.Task{doneTask, activeTask},
		NextShortID: 3,
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save original project: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load original project: %v", err)
	}

	const undoDepth = 5
	if err := storage.RecordUndoSnapshot(project, "Edit Task", "active-second", undoDepth, nil); err != nil {
		t.Fatalf("record undo snapshot: %v", err)
	}

	project.GetTaskByID(activeID).Status = models.StatusInProgress
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save current project: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load current project: %v", err)
	}

	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = intPtr(undoDepth)
	cfg.Projects = []*models.Project{project}
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		activePerspective: 1,
		loadedProjects:    []*models.Project{project},
	}

	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false in cross-project mode")
	}
	m = undoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Undid: Edit Task") {
		t.Fatalf("undo status = %q, want persistent undo", m.statusMessage)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load after undo: %v", err)
	}
	assertProjectTaskOrderAndStatus(t, project, activeID, models.StatusTodo)

	redoResult, _, ok := m.crossHandleRedo("ctrl+r")
	if !ok {
		t.Fatal("crossHandleRedo returned ok=false in cross-project mode")
	}
	m = redoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Redo: Edit Task") {
		t.Fatalf("redo status = %q, want persistent redo", m.statusMessage)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load after redo: %v", err)
	}
	assertProjectTaskOrderAndStatus(t, project, activeID, models.StatusInProgress)
}

func assertProjectTaskOrderAndStatus(t *testing.T, project *models.Project, firstTaskID string, firstStatus models.TaskStatus) {
	t.Helper()
	if len(project.Tasks) < 2 {
		t.Fatalf("project has %d tasks, want at least 2", len(project.Tasks))
	}
	if got := project.Tasks[0].ID; got != firstTaskID {
		t.Fatalf("first task ID = %q, want %q after post-restore reordering", got, firstTaskID)
	}
	if got := project.Tasks[0].Status; got != firstStatus {
		t.Fatalf("first task status = %s, want %s", got, firstStatus)
	}
}
