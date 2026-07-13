package app_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestDeleteRequiresExplicitConfirmation(t *testing.T) {
	task := newTask("t1", 1, "delete me")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.Delete(app.DeleteRequest{TaskID: "t1"})
	if !errors.Is(err, app.ErrDestructiveConfirmationRequired) {
		t.Fatalf("err = %v, want ErrDestructiveConfirmationRequired", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("unconfirmed delete committed %d times, want 0", len(repo.commits))
	}
	if len(project.Tasks) != 1 {
		t.Errorf("unconfirmed delete removed the task; project has %d tasks, want 1", len(project.Tasks))
	}
}

func TestDeleteRemovesSubtreeInOneChangeSet(t *testing.T) {
	parent := newTask("p", 1, "parent")
	child := newTask("c", 2, "child")
	childID := "p"
	child.ParentID = &childID
	project := newProject("proj-1", "Alpha", parent, child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Delete(app.DeleteRequest{TaskID: "p", Confirm: true})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(res.RemovedTaskIDs) != 2 {
		t.Fatalf("RemovedTaskIDs = %v, want 2 (parent + child)", res.RemovedTaskIDs)
	}
	if res.QualifiedRef != "alpha:#1" {
		t.Errorf("QualifiedRef = %q, want alpha:#1", res.QualifiedRef)
	}
	if len(project.Tasks) != 0 {
		t.Errorf("project has %d tasks, want 0 after subtree delete", len(project.Tasks))
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatalf("want exactly one changeset in one commit, got %v", repo.commits)
	}
	if cs.UndoAction != "Delete Task" {
		t.Errorf("UndoAction = %q, want Delete Task", cs.UndoAction)
	}
	if len(cs.AttachmentOps) != 2 {
		t.Fatalf("AttachmentOps = %d, want one AttachmentDelete per removed task", len(cs.AttachmentOps))
	}
	for _, op := range cs.AttachmentOps {
		if op.Kind != storage.AttachmentDelete {
			t.Errorf("attachment op kind = %d, want AttachmentDelete", op.Kind)
		}
		if op.SourceProject != project {
			t.Errorf("attachment op source project = %p, want %p", op.SourceProject, project)
		}
	}
}

func TestDeleteCleansCrossProjectRefsInOneTransaction(t *testing.T) {
	target := newTask("target", 1, "delete me")
	source := newProject("proj-1", "Alpha", target)

	// A task in another project depends on and relates to the deleted task via a
	// cross-project reference keyed by the source slug.
	dependent := newTask("dep", 1, "dependent")
	dependent.DependsOn = []string{"alpha:target"}
	dependent.RelatedTo = []string{"alpha:target"}
	other := newProject("proj-2", "Beta", dependent)

	repo := newFakeRepo(source, other)
	svc := newTestService(repo)

	res, err := svc.Delete(app.DeleteRequest{TaskID: "target", Confirm: true})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if res.CrossProjectRefsCleared != 2 {
		t.Errorf("CrossProjectRefsCleared = %d, want 2 (DependsOn + RelatedTo)", res.CrossProjectRefsCleared)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Errorf("TouchedProjectIDs = %v, want source + other", res.TouchedProjectIDs)
	}
	if res.TouchedProjectIDs[0] != "proj-1" {
		t.Errorf("first touched project = %q, want source proj-1", res.TouchedProjectIDs[0])
	}
	if len(dependent.DependsOn) != 0 || len(dependent.RelatedTo) != 0 {
		t.Errorf("cross-project refs survived: deps=%v related=%v", dependent.DependsOn, dependent.RelatedTo)
	}

	// The whole delete must be exactly one Commit call carrying two changesets.
	if len(repo.commits) != 1 {
		t.Fatalf("want one atomic commit, got %d commits", len(repo.commits))
	}
	if len(repo.commits[0]) != 2 {
		t.Fatalf("want two changesets in the single transaction, got %d", len(repo.commits[0]))
	}
}

func TestDeleteFoldsDurableArtifactIntoTransaction(t *testing.T) {
	task := newTask("t1", 1, "delete me")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// A caller-supplied durable artifact (e.g. a config store with the task's planning
	// refs cleaned) must ride the delete transaction and land in the commit.
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	art := storage.ArtifactChange{Path: cfgPath, Data: []byte(`{"focus_queue":[]}`)}

	if _, err := svc.Delete(app.DeleteRequest{TaskID: "t1", Confirm: true}, art); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatalf("want exactly one changeset in one commit, got %v", repo.commits)
	}
	if len(cs.Artifacts) != 1 || cs.Artifacts[0].Path != cfgPath {
		t.Fatalf("Artifacts = %v, want the supplied config artifact", cs.Artifacts)
	}
	if got := string(repo.files[filepath.Clean(cfgPath)]); got != `{"focus_queue":[]}` {
		t.Errorf("committed artifact bytes = %q, want cleaned config", got)
	}
}

func TestBulkDeleteFoldsDurableArtifactOntoFirstChangeset(t *testing.T) {
	a := newTask("a", 1, "a")
	b := newTask("b", 2, "b")
	project := newProject("proj-1", "Alpha", a, b)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	art := storage.ArtifactChange{Path: cfgPath, Data: []byte(`{"daily_plan":[]}`)}

	if _, err := svc.BulkDelete(app.BulkDeleteRequest{TaskIDs: []string{"a", "b"}, Confirm: true}, art); err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("want one atomic commit, got %d", len(repo.commits))
	}
	first := repo.commits[0][0]
	if len(first.Artifacts) != 1 || first.Artifacts[0].Path != cfgPath {
		t.Fatalf("first changeset Artifacts = %v, want the supplied config artifact", first.Artifacts)
	}
}

func TestDeleteUnknownTask(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	_, err := svc.Delete(app.DeleteRequest{TaskID: "ghost", Confirm: true})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestBulkDeleteRequiresConfirmation(t *testing.T) {
	a := newTask("a", 1, "a")
	b := newTask("b", 2, "b")
	project := newProject("proj-1", "Alpha", a, b)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.BulkDelete(app.BulkDeleteRequest{TaskIDs: []string{"a", "b"}})
	if !errors.Is(err, app.ErrDestructiveConfirmationRequired) {
		t.Fatalf("err = %v, want ErrDestructiveConfirmationRequired", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("unconfirmed bulk delete committed %d times, want 0", len(repo.commits))
	}
}

func TestBulkDeleteAcrossProjectsIsOneTransaction(t *testing.T) {
	a := newTask("a", 1, "a")
	pa := newProject("proj-1", "Alpha", a)
	b := newTask("b", 1, "b")
	pb := newProject("proj-2", "Beta", b)
	repo := newFakeRepo(pa, pb)
	svc := newTestService(repo)

	res, err := svc.BulkDelete(app.BulkDeleteRequest{TaskIDs: []string{"a", "b"}, Confirm: true})
	if err != nil {
		t.Fatalf("BulkDelete: %v", err)
	}
	if len(res.RemovedTaskIDs) != 2 {
		t.Errorf("RemovedTaskIDs = %v, want 2", res.RemovedTaskIDs)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Errorf("TouchedProjectIDs = %v, want both projects", res.TouchedProjectIDs)
	}
	if len(pa.Tasks) != 0 || len(pb.Tasks) != 0 {
		t.Errorf("tasks survived: alpha=%d beta=%d", len(pa.Tasks), len(pb.Tasks))
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("want one atomic commit of two changesets, got %v", repo.commits)
	}
}

func TestBulkTransitionAcrossProjectsIsOneTransaction(t *testing.T) {
	a := newTask("a", 1, "a")
	a.SetSection(models.SectionInbox)
	pa := newProject("proj-1", "Alpha", a)
	b := newTask("b", 1, "b")
	b.SetSection(models.SectionInbox)
	pb := newProject("proj-2", "Beta", b)
	repo := newFakeRepo(pa, pb)
	svc := newTestService(repo)

	res, err := svc.BulkTransition([]string{"a", "b"}, models.StateInProgress)
	if err != nil {
		t.Fatalf("BulkTransition: %v", err)
	}
	if len(res.Tasks) != 2 {
		t.Fatalf("Tasks = %d, want 2", len(res.Tasks))
	}
	if a.State() != models.StateInProgress || b.State() != models.StateInProgress {
		t.Errorf("states = %q/%q, want both in_progress", a.State(), b.State())
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("want one atomic commit of two changesets, got %v", repo.commits)
	}
}

func TestBulkTransitionAbortsWholeBatchOnInvalidTransition(t *testing.T) {
	a := newTask("a", 1, "a")
	a.SetSection(models.SectionInbox)
	terminal := newTask("done", 2, "already done")
	terminal.Complete()
	project := newProject("proj-1", "Alpha", a, terminal)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// Completing a task is fine, but transitioning a terminal task to in_progress
	// is forbidden — the whole batch must abort before any commit.
	_, err := svc.BulkTransition([]string{"a", "done"}, models.StateInProgress)
	if !errors.Is(err, app.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("aborted batch committed %d times, want 0", len(repo.commits))
	}
	if a.State() == models.StateInProgress {
		t.Errorf("task a mutated despite aborted batch: state = %q", a.State())
	}
}

func TestBulkTransitionRejectsUnknownTask(t *testing.T) {
	a := newTask("a", 1, "a")
	repo := newFakeRepo(newProject("proj-1", "Alpha", a))
	svc := newTestService(repo)

	_, err := svc.BulkTransition([]string{"a", "ghost"}, models.StateReady)
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("committed %d times despite unknown task, want 0", len(repo.commits))
	}
}
