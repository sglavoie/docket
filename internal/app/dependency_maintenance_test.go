package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// referenceTo is the locator shorthand these tests read a dependency by, so an
// assertion names the stable pair the document actually stores rather than a
// display value.
func referenceTo(projectID, taskID string) models.TaskReference {
	return models.TaskReference{ProjectID: projectID, TaskID: taskID}
}

// dependentTask seeds a task already holding stored references, which is the
// state every maintenance path starts from.
func dependentTask(id string, shortID int, title string, references ...models.TaskReference) *models.Task {
	task := newTask(id, shortID, title)
	task.Dependencies = references
	return task
}

func taskReferences(t *testing.T, project *models.Project, taskID string) []models.TaskReference {
	t.Helper()
	for _, task := range project.Tasks {
		if task.ID == taskID {
			return task.Dependencies
		}
	}
	t.Fatalf("project %s does not own task %s", project.ID, taskID)
	return nil
}

func requireReferences(t *testing.T, project *models.Project, taskID string, want ...models.TaskReference) {
	t.Helper()
	got := taskReferences(t, project, taskID)
	if !models.TaskReferencesEqual(got, want) {
		t.Fatalf("task %s references = %v, want %v", taskID, got, want)
	}
}

// committedProjectIDs reports the projects in the most recent commit batch, in
// commit order, which is how these tests prove a reference rewrite rode the same
// transaction as the move or delete that caused it.
func committedProjectIDs(repo *fakeRepo) []string {
	if len(repo.commits) == 0 {
		return nil
	}
	last := repo.commits[len(repo.commits)-1]
	ids := make([]string, 0, len(last))
	for _, cs := range last {
		ids = append(ids, cs.Project.ID)
	}
	return ids
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestCrossProjectMoveRewritesIncomingReferencesEverywhere proves the whole
// point of the pass: a task keeps its identity and changes address, so every
// document that named its old address is re-pointed in the same transaction —
// including a cataloged project the mutation snapshot never loaded.
func TestCrossProjectMoveRewritesIncomingReferencesEverywhere(t *testing.T) {
	source := newProject("proj-source", "Source", newTask("t-target", 1, "Target"))
	destination := newProject("proj-dest", "Dest")
	holder := newProject("proj-holder", "Holder",
		dependentTask("t-holder", 1, "Holder", referenceTo("proj-source", "t-target")))
	archived := newProject("proj-archived", "Archived",
		dependentTask("t-archived", 1, "Archived holder", referenceTo("proj-source", "t-target")))
	repo := newFakeRepo(source, destination, holder).withUnloaded(archived)
	svc := newTestService(repo)

	if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: "t-target"},
		Title:                "Target",
		DestinationProjectID: destination.ID,
	}); err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}

	requireReferences(t, holder, "t-holder", referenceTo("proj-dest", "t-target"))
	requireReferences(t, archived, "t-archived", referenceTo("proj-dest", "t-target"))

	committed := committedProjectIDs(repo)
	for _, want := range []string{"proj-source", "proj-dest", "proj-holder", "proj-archived"} {
		if !containsID(committed, want) {
			t.Fatalf("commit batch %v is missing project %s", committed, want)
		}
	}
}

// TestCrossProjectMoveRewritesReferencesInsideTheMovedSubtree covers the two
// halves of the outgoing rule at once: a reference between two co-moving tasks
// follows them to the destination, while a reference the subtree carries to a
// task that stayed behind is left exactly as it was.
func TestCrossProjectMoveRewritesReferencesInsideTheMovedSubtree(t *testing.T) {
	stay := newTask("t-stay", 1, "Stay")
	parent := dependentTask("t-parent", 2, "Parent", referenceTo("proj-source", "t-stay"))
	child := dependentTask("t-child", 3, "Child", referenceTo("proj-source", "t-parent"))
	child.ParentID = &parent.ID
	source := newProject("proj-source", "Source", stay, parent, child)
	destination := newProject("proj-dest", "Dest")
	repo := newFakeRepo(source, destination)
	svc := newTestService(repo)

	if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: parent.ID},
		Title:                "Parent",
		DestinationProjectID: destination.ID,
	}); err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}

	requireReferences(t, destination, "t-child", referenceTo("proj-dest", "t-parent"))
	requireReferences(t, destination, "t-parent", referenceTo("proj-source", "t-stay"))
}

// TestSameProjectMoveLeavesReferencesUntouched locks the fact that makes the
// pass cheap: a locator is a project and a task identity, and an internal
// reparent changes neither. The wider catalog is never even enumerated.
func TestSameProjectMoveLeavesReferencesUntouched(t *testing.T) {
	parent := newTask("t-parent", 1, "Parent")
	mover := dependentTask("t-mover", 2, "Mover", referenceTo("proj-source", "t-parent"))
	holder := dependentTask("t-holder", 3, "Holder", referenceTo("proj-source", "t-mover"))
	source := newProject("proj-source", "Source", parent, mover, holder)
	repo := newFakeRepo(source)
	svc := newTestService(repo)

	parentID := parent.ID
	if _, err := svc.Reparent(app.TaskLocator{ProjectID: source.ID, TaskID: mover.ID}, &parentID); err != nil {
		t.Fatalf("Reparent: %v", err)
	}

	requireReferences(t, source, "t-mover", referenceTo("proj-source", "t-parent"))
	requireReferences(t, source, "t-holder", referenceTo("proj-source", "t-mover"))
	if repo.referenceDocumentCalls != 0 {
		t.Fatalf("same-project move enumerated the catalog %d time(s)", repo.referenceDocumentCalls)
	}
}

// TestDeleteRemovesIncomingReferencesAcrossTheCatalog proves a deleted subtree
// takes every reference to it along, in the same transaction, from loaded and
// unloaded documents alike — and that a reference to a surviving sibling is not
// collateral damage.
func TestDeleteRemovesIncomingReferencesAcrossTheCatalog(t *testing.T) {
	survivor := newTask("t-survivor", 1, "Survivor")
	root := newTask("t-root", 2, "Root")
	child := newTask("t-child", 3, "Child")
	child.ParentID = &root.ID
	source := newProject("proj-source", "Source", survivor, root, child)
	holder := newProject("proj-holder", "Holder", dependentTask(
		"t-holder", 1, "Holder",
		referenceTo("proj-source", "t-root"),
		referenceTo("proj-source", "t-survivor"),
	))
	archived := newProject("proj-archived", "Archived", dependentTask(
		"t-archived", 1, "Archived holder",
		referenceTo("proj-source", "t-child"),
	))
	repo := newFakeRepo(source, holder).withUnloaded(archived)
	svc := newTestService(repo)

	if _, err := svc.Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Confirm: true,
	}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	requireReferences(t, holder, "t-holder", referenceTo("proj-source", "t-survivor"))
	requireReferences(t, archived, "t-archived")
	committed := committedProjectIDs(repo)
	for _, want := range []string{"proj-source", "proj-holder", "proj-archived"} {
		if !containsID(committed, want) {
			t.Fatalf("commit batch %v is missing project %s", committed, want)
		}
	}
}

// TestBulkDeleteRemovesIncomingReferencesFromEverySourceProject exercises the
// batch spelling: two subtrees in two projects leave together and one holder
// loses both references in one transaction.
func TestBulkDeleteRemovesIncomingReferencesFromEverySourceProject(t *testing.T) {
	first := newProject("proj-first", "First", newTask("t-first", 1, "First"))
	second := newProject("proj-second", "Second", newTask("t-second", 1, "Second"))
	holder := newProject("proj-holder", "Holder", dependentTask(
		"t-holder", 1, "Holder",
		referenceTo("proj-first", "t-first"),
		referenceTo("proj-second", "t-second"),
	))
	repo := newFakeRepo(first, second, holder)
	svc := newTestService(repo)

	if _, err := svc.BulkDelete(app.BulkDeleteRequest{
		TaskLocators: []app.TaskLocator{
			{ProjectID: first.ID, TaskID: "t-first"},
			{ProjectID: second.ID, TaskID: "t-second"},
		},
		Confirm: true,
	}); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}

	requireReferences(t, holder, "t-holder")
}

// TestReferenceMaintenanceRollsBackWhenTheCommitConflicts proves failure is
// atomic across every touched project: a stale revision aborts the transaction
// and the holder's stored references are left exactly as they were rather than
// half repaired.
func TestReferenceMaintenanceRollsBackWhenTheCommitConflicts(t *testing.T) {
	source := newProject("proj-source", "Source", newTask("t-target", 1, "Target"))
	holder := newProject("proj-holder", "Holder",
		dependentTask("t-holder", 1, "Holder", referenceTo("proj-source", "t-target")))
	repo := newFakeRepo(source, holder)
	repo.preCommit = func() { repo.revisions["proj-holder"] = "moved-on" }
	svc := newTestService(repo)

	_, err := svc.Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: source.ID, TaskID: "t-target"},
		Confirm: true,
	})
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("Delete error = %v, want a stale revision conflict", err)
	}
	requireReferences(t, holder, "t-holder", referenceTo("proj-source", "t-target"))
	if len(source.Tasks) != 1 {
		t.Fatalf("source tasks after refused delete = %d, want 1", len(source.Tasks))
	}
}

// TestReferenceMaintenanceFailsClosedOnAnUnreadableDocument locks the posture
// the integrity contract requires: a catalog document nobody can read is a
// document whose stale locators nobody can repair, so the whole mutation is
// refused rather than committed with a known blind spot.
func TestReferenceMaintenanceFailsClosedOnAnUnreadableDocument(t *testing.T) {
	source := newProject("proj-source", "Source", newTask("t-target", 1, "Target"))
	repo := newFakeRepo(source)
	repo.referenceErr = errors.New("archived document is unreadable")
	svc := newTestService(repo)

	_, err := svc.Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: source.ID, TaskID: "t-target"},
		Confirm: true,
	})
	if err == nil {
		t.Fatal("Delete succeeded despite an unreadable catalog document")
	}
	if len(repo.commits) != 0 {
		t.Fatalf("refused delete committed %d transaction(s)", len(repo.commits))
	}
	if len(source.Tasks) != 1 {
		t.Fatalf("source tasks after refused delete = %d, want 1", len(source.Tasks))
	}
}

// TestDeleteProjectRemovesEveryReferenceTargetingIt covers the third destructive
// shape. Project deletion commits raw filesystem choreography, so each reference
// rewrite rides as a revision-checked tasks.md artifact in that same transaction.
func TestDeleteProjectRemovesEveryReferenceTargetingIt(t *testing.T) {
	dataDir := t.TempDir()
	doomed := seedProjectDir(t, dataDir, "doomed", "Doomed")
	holder := seedProjectDir(t, dataDir, "holder", "Holder")
	holder.Tasks = []*models.Task{dependentTask(
		"t-holder", 1, "Holder",
		referenceTo(doomed.ID, "t-doomed"),
		referenceTo("proj-elsewhere", "t-elsewhere"),
	)}
	holder.NextShortID = 2
	holder.BuildTaskTree()

	repo := &opsRepo{fakeRepo: newFakeRepo(holder)}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	if _, err := svc.DeleteProjectResult(app.DeleteProjectRequest{
		Project: doomed,
		DataDir: dataDir,
	}); err != nil {
		t.Fatalf("DeleteProjectResult: %v", err)
	}

	if len(repo.committedArtifacts) != 1 {
		t.Fatalf("delete committed %d artifact batches, want 1", len(repo.committedArtifacts))
	}
	tasksPath := storage.TasksFilePath(holder.Path)
	found := false
	for _, artifact := range repo.committedArtifacts[0] {
		if artifact.Path != tasksPath {
			continue
		}
		found = true
		if artifact.ExpectedRevision == nil {
			t.Fatal("reference rewrite committed without an expected revision")
		}
		document, err := storage.ParseProjectDocument(string(artifact.Data))
		if err != nil {
			t.Fatalf("ParseProjectDocument: %v", err)
		}
		if len(document.Tasks) != 1 ||
			!models.TaskReferencesEqual(document.Tasks[0].Dependencies,
				[]models.TaskReference{referenceTo("proj-elsewhere", "t-elsewhere")}) {
			t.Fatalf("rewritten references = %v", document.Tasks[0].Dependencies)
		}
	}
	if !found {
		t.Fatalf("project delete did not rewrite %s", tasksPath)
	}
	if got := taskReferences(t, holder, "t-holder"); len(got) != 2 {
		t.Fatalf("in-memory holder references = %v, want the pre-commit pair", got)
	}
}

// TestArchivingAProjectLeavesReferencesUntouched holds the negative half of the
// rule apart from the positive one: archiving changes a project's visibility,
// never its identity, so nothing about a stored locator becomes wrong and the
// documents that hold one are not rewritten.
func TestArchivingAProjectLeavesReferencesUntouched(t *testing.T) {
	target := newProject("proj-target", "Target", newTask("t-target", 1, "Target"))
	holder := newProject("proj-holder", "Holder",
		dependentTask("t-holder", 1, "Holder", referenceTo("proj-target", "t-target")))
	repo := newFakeRepo(holder).withUnloaded(target)
	svc := newTestService(repo)

	// The archived target is exactly the population an ordinary mutation cannot
	// see. Committing an unrelated edit in the holder must neither repair nor
	// prune the reference pointing into it.
	if _, err := svc.SetPriority(app.TaskLocator{ProjectID: holder.ID, TaskID: "t-holder"}, 2); err != nil {
		t.Fatalf("SetPriority: %v", err)
	}
	requireReferences(t, holder, "t-holder", referenceTo("proj-target", "t-target"))
}

// TestCrossProjectMoveReferenceUndoAcceptance drives the whole wired path
// through real storage: the archived document's rewrite lands in the same
// journaled transaction as the move, and one workspace ledger undo puts its
// bytes back. Undo replays the recorded images; it never reruns the pass.
func TestCrossProjectMoveReferenceUndoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

	source := realTestProject(t, dataDir, "source", "Target")
	destination := realTestProject(t, dataDir, "dest", "Anchor")
	archived := realTestProject(t, dataDir, "archived", "Archived holder")
	target := source.Tasks[0]
	archived.Tasks[0].Dependencies = []models.TaskReference{referenceTo(source.ID, target.ID)}

	repo := newStorageRepo(commitSvc, source, destination, archived)
	repo.seed(t)
	// Archived projects are cataloged but never loaded into a mutation snapshot;
	// only the reference-document source sees this one from here on.
	repo.hideProject(archived.ID)
	archivedTasks := storage.TasksFilePath(archived.Path)
	before := readFileContent(t, archivedTasks)

	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: target.ID},
		Title:                target.Title,
		DestinationProjectID: destination.ID,
	}); err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}

	moved := readFileContent(t, archivedTasks)
	if moved == before {
		t.Fatal("archived document was not rewritten by the move")
	}
	document, err := storage.ParseProjectDocument(moved)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if !models.TaskReferencesEqual(document.Tasks[0].Dependencies,
		[]models.TaskReference{referenceTo(destination.ID, target.ID)}) {
		t.Fatalf("rewritten archived references = %v", document.Tasks[0].Dependencies)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := readFileContent(t, archivedTasks); got != before {
		t.Fatalf("undo restored archived document as %q, want %q", got, before)
	}
	if _, err := os.Stat(storage.TasksFilePath(source.Path)); err != nil {
		t.Fatalf("source document after undo: %v", err)
	}
}
