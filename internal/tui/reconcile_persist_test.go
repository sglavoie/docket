package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// newDoneParentDoneChildProject builds a project persisted to a temp dir with a
// done Story parent and a done Task child (a consistent tree), then reloads it
// from disk. Returns the reloaded project, parent ID, and child ID.
func newDoneParentDoneChildProject(t *testing.T) (*models.Project, string, string) {
	t.Helper()

	parent, err := models.NewTask("parent-done-1", 1, models.TaskTypeStory, "Parent Story")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parent.Complete()

	child, err := models.NewTask("child-done-1", 2, models.TaskTypeTask, "Child Task")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	child.ParentID = &parent.ID
	child.Complete()

	project := &models.Project{
		ID:          "reconcile-proj",
		Name:        "Reconcile",
		Path:        t.TempDir(),
		Tasks:       []*models.Task{parent, child},
		NextShortID: 3,
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load project: %v", err)
	}
	return project, parent.ID, child.ID
}

// TestSimplePersistWithUndoReopensParentAndUndoRestoresBoth verifies that
// reopening a child of a done parent through a simplePersistWithUndo-backed op
// reconciles the parent open on disk, and a single undo restores BOTH the child
// and the reopened parent (the widening contract).
func TestSimplePersistWithUndoReopensParentAndUndoRestoresBoth(t *testing.T) {
	project, parentID, childID := newDoneParentDoneChildProject(t)

	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = intPtr(5)
	p := NewProjectService()

	pf := simplePersistWithUndo(p, cfg, "Reopen Task")
	res := pf(project, childID, func(task *models.Task) { task.Uncomplete() })
	if res.Err != nil {
		t.Fatalf("persist returned error: %v", res.Err)
	}

	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload after persist: %v", err)
	}
	if got := project.GetTaskByID(childID).Status; got.IsComplete() {
		t.Errorf("child status after reopen = %s, want incomplete", got)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusInProgress {
		t.Errorf("parent status after reconcile = %s, want %s", got, models.StatusInProgress)
	}

	// Undo widening: the single snapshot must target both child and parent.
	history, err := storage.LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("load undo history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 undo entry, got %d", len(history))
	}
	if !containsID(history[0].TaskIDs, childID) || !containsID(history[0].TaskIDs, parentID) {
		t.Fatalf("undo TaskIDs = %v, want both %q and %q", history[0].TaskIDs, childID, parentID)
	}

	if _, err := storage.PerformUndo(project); err != nil {
		t.Fatalf("perform undo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload after undo: %v", err)
	}
	if got := project.GetTaskByID(childID).Status; got != models.StatusDone {
		t.Errorf("child status after undo = %s, want %s", got, models.StatusDone)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusDone {
		t.Errorf("parent status after undo = %s, want %s (single undo must restore the reopened parent)", got, models.StatusDone)
	}
}

// TestCrossPersistFnReopensParentAndFullFileUndoRestores verifies the
// cross-project single-mutation path (crossPersistFn) reopens the parent on
// disk and a full-file undo restores the pre-mutation state.
func TestCrossPersistFnReopensParentAndFullFileUndoRestores(t *testing.T) {
	project, parentID, childID := newDoneParentDoneChildProject(t)

	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = intPtr(5)
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		loadedProjects:    []*models.Project{project},
	}
	m.crossUndoCtx.Set("Reopen Task", "cross reopen")
	m.crossUndoCtx.ProjectIDs = []string{project.ID}

	pf := m.crossPersistFn(project)
	res := pf(project, childID, func(task *models.Task) { task.Uncomplete() })
	if res.Err != nil {
		t.Fatalf("cross persist returned error: %v", res.Err)
	}

	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload after cross persist: %v", err)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusInProgress {
		t.Errorf("parent status after reconcile = %s, want %s", got, models.StatusInProgress)
	}

	if _, err := storage.PerformUndo(project); err != nil {
		t.Fatalf("perform undo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload after undo: %v", err)
	}
	if got := project.GetTaskByID(childID).Status; got != models.StatusDone {
		t.Errorf("child status after undo = %s, want %s", got, models.StatusDone)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusDone {
		t.Errorf("parent status after undo = %s, want %s", got, models.StatusDone)
	}
}

// TestSimplePersistWithUndoNoSpuriousReopen verifies reconciliation does not
// fire when the tree stays consistent: mutating a leaf's priority on an
// all-incomplete tree reopens nothing and does not pollute the undo set.
func TestSimplePersistWithUndoNoSpuriousReopen(t *testing.T) {
	parent, err := models.NewTask("parent-open-1", 1, models.TaskTypeStory, "Parent Story")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parent.Status = models.StatusInProgress
	child, err := models.NewTask("child-open-1", 2, models.TaskTypeTask, "Child Task")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	child.ParentID = &parent.ID
	child.Status = models.StatusTodo

	project := &models.Project{
		ID:          "consistent-proj",
		Name:        "Consistent",
		Path:        t.TempDir(),
		Tasks:       []*models.Task{parent, child},
		NextShortID: 3,
	}
	project.BuildTaskTree()
	if err = storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err = storage.LoadProject(project); err != nil {
		t.Fatalf("load project: %v", err)
	}

	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = intPtr(5)
	p := NewProjectService()

	pf := simplePersistWithUndo(p, cfg, "Set Priority")
	res := pf(project, child.ID, func(task *models.Task) { task.Priority = 1 })
	if res.Err != nil {
		t.Fatalf("persist returned error: %v", res.Err)
	}

	if err = storage.LoadProject(project); err != nil {
		t.Fatalf("reload after persist: %v", err)
	}
	if got := project.GetTaskByID(parent.ID).Status; got != models.StatusInProgress {
		t.Errorf("parent status = %s, want unchanged %s (no spurious reopen)", got, models.StatusInProgress)
	}

	history, err := storage.LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("load undo history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 undo entry, got %d", len(history))
	}
	if len(history[0].TaskIDs) != 1 || history[0].TaskIDs[0] != child.ID {
		t.Errorf("undo TaskIDs = %v, want only [%q] (no undo-set pollution)", history[0].TaskIDs, child.ID)
	}
}

// TestPersistWithWorkflowPostRuleReconcileReopensParent verifies the post-rule
// reconcile: a workflow rule that completes a parent AFTER a child is reopened
// (a complete_parent action fired by the "reopened" event) cannot save the
// illegal parent-done-with-incomplete-child state. The initial reconcile reopens
// the parent, the rule re-completes it, and the second ReconcileParentCompletion
// in the post-rule hook block reopens it again so the saved file is consistent.
// A single undo restores both the child and the reopened parent.
func TestPersistWithWorkflowPostRuleReconcileReopensParent(t *testing.T) {
	project, parentID, childID := newDoneParentDoneChildProject(t)

	cfg := config.NewConfig()
	cfg.UndoHistoryDepth = intPtr(5)
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{
			ID:      "reopen-completes-parent",
			Name:    "Complete parent when child reopened",
			Enabled: true,
			Source:  "user",
			Trigger: workflow.RuleTrigger{
				Field:     workflow.FieldReopened,
				Operation: workflow.OpEquals,
				Value:     "true",
			},
			Actions: []workflow.RuleAction{
				{ActionType: workflow.ActionCompleteParent},
			},
		},
	}
	p := NewProjectService()

	// Capture pre-mutation states so the "reopened" event trigger can fire,
	// then reopen the child through the full workflow pipeline.
	prevStates := captureTaskStates(project.Tasks)
	project.MutateTask(childID, func(task *models.Task) { task.Uncomplete() })

	if err := persistWithWorkflow(WorkflowSaveOptions{
		Persister:   p,
		Project:     project,
		Config:      cfg,
		PrevStates:  prevStates,
		UndoAction:  "Reopen Task",
		UndoContext: "#2 Child",
		UndoDepth:   *cfg.UndoHistoryDepth,
		UndoTaskIDs: []string{childID},
	}); err != nil {
		t.Fatalf("persistWithWorkflow: %v", err)
	}

	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload after persist: %v", err)
	}
	if got := project.GetTaskByID(childID).Status; got.IsComplete() {
		t.Errorf("child status after reopen = %s, want incomplete", got)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusInProgress {
		t.Errorf("parent status after post-rule reconcile = %s, want %s (rule must not save an illegal done parent)", got, models.StatusInProgress)
	}

	// Undo widening: the single snapshot must target both the child and the
	// parent reopened by the post-rule reconcile.
	history, err := storage.LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("load undo history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 undo entry, got %d", len(history))
	}
	if !containsID(history[0].TaskIDs, childID) || !containsID(history[0].TaskIDs, parentID) {
		t.Fatalf("undo TaskIDs = %v, want both %q and %q", history[0].TaskIDs, childID, parentID)
	}

	if _, err := storage.PerformUndo(project); err != nil {
		t.Fatalf("perform undo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("reload after undo: %v", err)
	}
	if got := project.GetTaskByID(childID).Status; got != models.StatusDone {
		t.Errorf("child status after undo = %s, want %s", got, models.StatusDone)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusDone {
		t.Errorf("parent status after undo = %s, want %s (single undo must restore the reopened parent)", got, models.StatusDone)
	}
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
