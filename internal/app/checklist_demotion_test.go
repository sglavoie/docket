package app_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/util"
)

func demotionService(t *testing.T, owner, child *models.Task) (*fakeRepo, *app.Service) {
	t.Helper()
	repo := newFakeRepo(newProject("p1", "Alpha", owner, child))
	svc := newTestService(repo).WithPlanningArtifacts(func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		if req.RequireAbsent == nil || !req.GuardOnly {
			t.Fatal("demotion did not guard planning")
		}
		absent := ""
		return []storage.ArtifactChange{{Path: "/planning.json", Data: []byte("{}"), ExpectedRevision: &absent}}, nil
	})
	return repo, svc
}

func frozenChild(t *testing.T, child *models.Task) string {
	t.Helper()
	data, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMakeChecklistItemPlacementAndState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		origin *models.ChecklistOrigin
		state  models.WorkflowState
		index  int
		done   bool
	}{
		{"ordinary", nil, models.StateBacklog, 2, false},
		{"older promoted without origin", nil, models.StateDone, 2, true},
		{"promoted", &models.ChecklistOrigin{ProjectID: "p1", TaskID: "owner", Index: 1}, models.StateBacklog, 1, false},
		{"clamped", &models.ChecklistOrigin{ProjectID: "p1", TaskID: "owner", Index: 99}, models.StateDone, 2, true},
		{"moved", &models.ChecklistOrigin{ProjectID: "p1", TaskID: "previous", Index: 0}, models.StateDone, 2, true},
		{"moved project", &models.ChecklistOrigin{ProjectID: "previous", TaskID: "owner", Index: 0}, models.StateBacklog, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := newTask("owner", 1, "Owner")
			owner.Checklist = []models.ChecklistItem{{Text: "first"}, {Text: "second"}}
			child := newTask("child", 2, "Current title")
			child.ParentID = &owner.ID
			child.ChecklistOrigin = tc.origin
			child.WorkflowState = tc.state
			if tc.state == models.StateDone {
				completed := child.CreatedAt
				child.CompletedAt = &completed
			}
			repo, svc := demotionService(t, owner, child)
			result, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
			if err != nil {
				t.Fatal(err)
			}
			if result.Index != tc.index || len(owner.Checklist) != 3 || owner.Checklist[tc.index] != (models.ChecklistItem{Text: "Current title", Done: tc.done}) {
				t.Fatalf("result=%+v checklist=%+v", result, owner.Checklist)
			}
			if repo.projects[0].GetTaskByID("child") != nil || len(repo.commits) != 1 {
				t.Fatalf("child or transaction remained: %v %d", repo.projects[0].GetTaskByID("child"), len(repo.commits))
			}
		})
	}
}

func TestMakeChecklistItemRefusesLossAndStaleRow(t *testing.T) {
	// A checklist item holds a title and a tick, so a check-in demoted with its
	// task would stop raising a reminder nobody withdrew and a review instant would
	// be lost with no record it was ever answered. Both are refusals rather than
	// silent drops for that reason.
	checkIn, err := util.StartOfCalendarDate(2026, time.September, 1, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name   string
		mutate func(*models.Task, *models.Task)
	}{
		{"description", func(_, c *models.Task) { c.Description = "keep" }},
		{"checklist", func(_, c *models.Task) { c.Checklist = []models.ChecklistItem{{Text: "nested"}} }},
		{"no parent", func(_, c *models.Task) { c.ParentID = nil }},
		{"working", func(_, c *models.Task) { c.WorkflowState = models.StateDoing }},
		{"check-in", func(_, c *models.Task) { c.NextCheckIn = &checkIn; c.CheckInRepeatDays = 7 }},
		{"review history", func(_, c *models.Task) { c.LastReviewedAt = &reviewed }},
		{"hidden", func(_, c *models.Task) { c.Hidden = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := newTask("owner", 1, "Owner")
			child := newTask("child", 2, "Child")
			child.ParentID = &owner.ID
			tc.mutate(owner, child)
			repo, svc := demotionService(t, owner, child)
			before := frozenChild(t, child)
			_, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: before})
			if err == nil || len(owner.Checklist) != 0 || len(repo.commits) != 0 || repo.projects[0].GetTaskByID("child") == nil {
				t.Fatalf("refusal=%v", err)
			}
		})
	}
	owner := newTask("owner", 1, "Owner")
	child := newTask("child", 2, "Child")
	child.ParentID = &owner.ID
	repo, svc := demotionService(t, owner, child)
	frozen := frozenChild(t, child)
	child.Title = "edited"
	if _, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozen}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale row: %v", err)
	}
}

func TestMakeChecklistItemRefusesIncomingReferenceAndConcurrentChange(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	child := newTask("child", 2, "Child")
	child.ParentID = &owner.ID
	repo, svc := demotionService(t, owner, child)
	holder := newTask("holder", 3, "Holder")
	holder.Dependencies = []models.TaskReference{{ProjectID: "p1", TaskID: "child"}}
	repo.projects[0].Tasks = append(repo.projects[0].Tasks, holder)
	_, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
	if err == nil || !strings.Contains(err.Error(), "references") || len(repo.commits) != 0 {
		t.Fatalf("incoming reference refusal: %v", err)
	}
	holder.Dependencies = nil
	repo.preCommit = func() { repo.revisions["p1"] = "external revision" }
	_, err = svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
	if err == nil || len(repo.commits) != 0 || len(owner.Checklist) != 0 || repo.projects[0].GetTaskByID("child") == nil {
		t.Fatalf("concurrent edit refusal: %v", err)
	}
}

func TestMakeChecklistItemRefusesArchivedIncomingReference(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	child := newTask("child", 2, "Child")
	child.ParentID = &owner.ID
	archivedHolder := newTask("holder", 1, "Archived holder")
	archivedHolder.Dependencies = []models.TaskReference{{ProjectID: "p1", TaskID: "child"}}
	repo := newFakeRepo(newProject("p1", "Alpha", owner, child)).withUnloaded(newProject("p2", "Archived", archivedHolder))
	svc := newTestService(repo)
	_, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
	if err == nil || !strings.Contains(err.Error(), "references") || len(repo.commits) != 0 {
		t.Fatalf("archived reference refusal: %v", err)
	}
}

func TestMakeChecklistItemRefusesDescendants(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	child := newTask("child", 2, "Child")
	child.ParentID = &owner.ID
	grandchild := newTask("grandchild", 3, "Grandchild")
	grandchild.ParentID = &child.ID
	repo := newFakeRepo(newProject("p1", "Alpha", owner, child, grandchild))
	svc := newTestService(repo)
	_, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
	if err == nil || !strings.Contains(err.Error(), "descendants") || len(repo.commits) != 0 {
		t.Fatalf("descendant refusal: %v", err)
	}
}

func TestMakeChecklistItemRefusesConcurrentPlanningWrite(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	child := newTask("child", 2, "Child")
	child.ParentID = &owner.ID
	repo, svc := demotionService(t, owner, child)
	repo.validateChangeArtifacts = true
	repo.preCommit = func() { repo.files["/planning.json"] = []byte("new planning membership") }
	_, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
	if err == nil || len(repo.commits) != 0 || len(owner.Checklist) != 0 || repo.projects[0].GetTaskByID("child") == nil {
		t.Fatalf("concurrent planning refusal: %v", err)
	}
}

func TestMakeChecklistItemRefusesConcurrentCatalogChange(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	child := newTask("child", 2, "Child")
	child.ParentID = &owner.ID
	repo, svc := demotionService(t, owner, child)
	repo.preCommit = func() { repo.files[fakeCatalogPath] = []byte("new project added") }
	_, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc("child"), FrozenTaskJSON: frozenChild(t, child)})
	if err == nil || len(repo.commits) != 0 || len(owner.Checklist) != 0 || repo.projects[0].GetTaskByID("child") == nil {
		t.Fatalf("concurrent catalog refusal: %v", err)
	}
}

func TestMakeChecklistItemUndoRedoRestoresBothSides(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := realTestProject(t, dataDir, "alpha", "Owner")
	owner := project.Tasks[0]
	owner.Checklist = []models.ChecklistItem{{Text: "first"}, {Text: "last"}}
	child := newTask("child", project.GetNextShortID(), "Middle")
	child.ParentID = &owner.ID
	child.ChecklistOrigin = &models.ChecklistOrigin{ProjectID: project.ID, TaskID: owner.ID, Index: 1}
	project.Tasks = append(project.Tasks, child)
	project.BuildTaskTree()
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	absent := ""
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil).WithPlanningArtifacts(func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		return []storage.ArtifactChange{{Path: filepath.Join(dataDir, "planning-guard.json"), Data: []byte("{}"), ExpectedRevision: &absent}}, nil
	})
	if _, err := svc.MakeChecklistItem(app.ChecklistDemotionRequest{Child: repo.loc(child.ID), FrozenTaskJSON: frozenChild(t, child)}); err != nil {
		t.Fatal(err)
	}
	after := reloadPromotionProject(t, dataDir, project)
	if after.GetTaskByID(child.ID) != nil || len(after.GetTaskByID(owner.ID).Checklist) != 3 {
		t.Fatal("demotion did not commit both sides")
	}
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatal(err)
	}
	undone := reloadPromotionProject(t, dataDir, project)
	if undone.GetTaskByID(child.ID) == nil || len(undone.GetTaskByID(owner.ID).Checklist) != 2 {
		t.Fatal("undo failed to restore both sides")
	}
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatal(err)
	}
	redone := reloadPromotionProject(t, dataDir, project)
	if redone.GetTaskByID(child.ID) != nil || len(redone.GetTaskByID(owner.ID).Checklist) != 3 {
		t.Fatal("redo failed to remove the child and restore the item")
	}
}
