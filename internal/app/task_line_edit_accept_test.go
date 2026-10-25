package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestTaskLineEditUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Before")
	task := project.Tasks[0]
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	result, err := app.NewServiceWithArtifacts(repo, nil, nil, nil).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "After",
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.TransactionID == "" {
		t.Fatal("committed edit did not return a transaction ID")
	}
	if got := loadTaskLineEditTitle(t, project, task.ID); got != "After" {
		t.Fatalf("committed title = %q, want After", got)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := loadTaskLineEditTitle(t, project, task.ID); got != "Before" {
		t.Fatalf("undone title = %q, want Before", got)
	}

	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := loadTaskLineEditTitle(t, project, task.ID); got != "After" {
		t.Fatalf("redone title = %q, want After", got)
	}
}

func TestTaskLineEditCrossProjectUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	source := realTestProject(t, dataDir, "source", "Before")
	root := source.Tasks[0]
	child, err := models.NewGenericTask(uuid.New().String(), 2, "Child")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	child.ParentID = &root.ID
	source.Tasks = append(source.Tasks, child)
	source.NextShortID = 3
	source.BuildTaskTree()
	destination := realTestProject(t, dataDir, "destination", "Destination parent")
	parent := destination.Tasks[0]

	planningPath := filepath.Join(dataDir, ".docket", "planning.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(planningPath), 0o755); mkdirErr != nil {
		t.Fatalf("mkdir planning dir: %v", mkdirErr)
	}
	beforePlanning := []byte("source-planning")
	if writeErr := os.WriteFile(planningPath, beforePlanning, 0o600); writeErr != nil {
		t.Fatalf("seed planning: %v", writeErr)
	}
	expectedPlanning := storage.RawArtifactRevision(beforePlanning)

	repo := newStorageRepo(commitSvc, source, destination)
	repo.seed(t)
	sourceAttachment := seedRealAttachment(t, source, child, "child.txt", []byte("child attachment"))
	destinationAttachmentDir, err := storage.ResolveTaskAttachmentsDir(destination, child)
	if err != nil {
		t.Fatalf("ResolveTaskAttachmentsDir destination: %v", err)
	}
	destinationAttachment := filepath.Join(destinationAttachmentDir, "child.txt")

	var planningRequest app.PlanningReconcileRequest
	result, err := app.NewServiceWithArtifacts(repo, nil, nil, nil).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Title:                "After",
		DestinationProjectID: destination.ID,
		Parent: app.TaskLineParentPatch{
			Mode:   app.TaskLineParentSet,
			Parent: app.TaskLocator{ProjectID: destination.ID, TaskID: parent.ID},
		},
		PlanningArtifacts: func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
			planningRequest = req
			return []storage.ArtifactChange{{
				Path: planningPath, Data: []byte("destination-planning"), ExpectedRevision: &expectedPlanning,
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.TransactionID == "" || result.Task.ProjectID != destination.ID || len(planningRequest.Moves) != 2 {
		t.Fatalf("cross-project result/planning = %+v / %+v", result, planningRequest)
	}
	assertTaskLineEditOwnership(t, source, destination, root.ID, destination.ID)
	requireAbsentFile(t, sourceAttachment)
	if got := readFileContent(t, destinationAttachment); got != "child attachment" {
		t.Fatalf("moved attachment = %q", got)
	}
	if got := readFileContent(t, planningPath); got != "destination-planning" {
		t.Fatalf("planning after edit = %q", got)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	assertTaskLineEditOwnership(t, source, destination, root.ID, source.ID)
	if got := readFileContent(t, sourceAttachment); got != "child attachment" {
		t.Fatalf("restored attachment = %q", got)
	}
	requireAbsentFile(t, destinationAttachment)
	if got := readFileContent(t, planningPath); got != "source-planning" {
		t.Fatalf("planning after undo = %q", got)
	}

	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	assertTaskLineEditOwnership(t, source, destination, root.ID, destination.ID)
	requireAbsentFile(t, sourceAttachment)
	if got := readFileContent(t, destinationAttachment); got != "child attachment" {
		t.Fatalf("redone attachment = %q", got)
	}
	if got := readFileContent(t, planningPath); got != "destination-planning" {
		t.Fatalf("planning after redo = %q", got)
	}
}

func TestTaskLineEditAttachmentRelocationFailureLeavesDocumentsUntouched(t *testing.T) {
	dataDir := t.TempDir()
	commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir})
	source := realTestProject(t, dataDir, "source", "Before")
	root := source.Tasks[0]
	destination := realTestProject(t, dataDir, "destination", "Destination task")
	repo := newStorageRepo(commitSvc, source, destination)
	repo.seed(t)

	sourceAttachment := seedRealAttachment(t, source, root, "source.txt", []byte("source attachment"))
	destinationDir, err := storage.ResolveTaskAttachmentsDir(destination, root)
	if err != nil {
		t.Fatalf("ResolveTaskAttachmentsDir: %v", err)
	}
	if mkdirErr := os.MkdirAll(destinationDir, 0o755); mkdirErr != nil {
		t.Fatalf("mkdir destination collision: %v", mkdirErr)
	}
	collision := filepath.Join(destinationDir, "collision.txt")
	if writeErr := os.WriteFile(collision, []byte("occupied"), 0o600); writeErr != nil {
		t.Fatalf("write destination collision: %v", writeErr)
	}

	_, err = app.NewServiceWithArtifacts(repo, nil, nil, nil).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Title:                "After",
		DestinationProjectID: destination.ID,
	})
	if err == nil {
		t.Fatal("expected attachment relocation failure")
	}
	if root.Title != "Before" || source.GetTaskByID(root.ID) != root || destination.GetTaskByID(root.ID) != nil {
		t.Fatalf("in-memory rollback failed: root=%+v source=%+v destination=%+v", root, source.Tasks, destination.Tasks)
	}
	assertTaskLineEditOwnership(t, source, destination, root.ID, source.ID)
	if got := readFileContent(t, sourceAttachment); got != "source attachment" {
		t.Fatalf("source attachment after failure = %q", got)
	}
	if got := readFileContent(t, collision); got != "occupied" {
		t.Fatalf("destination collision after failure = %q", got)
	}
}

func assertTaskLineEditOwnership(t *testing.T, source, destination *models.Project, taskID, ownerID string) {
	t.Helper()
	sourceReloaded := loadTaskLineEditProject(t, source)
	destinationReloaded := loadTaskLineEditProject(t, destination)
	if got := sourceReloaded.GetTaskByID(taskID) != nil; got != (ownerID == source.ID) {
		t.Fatalf("source ownership = %t, want owner %s", got, ownerID)
	}
	if got := destinationReloaded.GetTaskByID(taskID) != nil; got != (ownerID == destination.ID) {
		t.Fatalf("destination ownership = %t, want owner %s", got, ownerID)
	}
}

func loadTaskLineEditProject(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	return reloaded
}

func loadTaskLineEditTitle(t *testing.T, project *models.Project, taskID string) string {
	t.Helper()
	reloaded := loadTaskLineEditProject(t, project)
	task := reloaded.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("reloaded task %s is missing", taskID)
	}
	return task.Title
}
