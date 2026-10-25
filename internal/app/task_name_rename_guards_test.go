package app_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/storage"
)

// TestRenameTaskNameGuardsTheWholeCensusAndCatalog inspects the emitted commit:
// only changed projects become changesets, every other census project rides as a
// read-only project guard, and the catalog the census enumerated is guarded too.
func TestRenameTaskNameGuardsTheWholeCensusAndCatalog(t *testing.T) {
	alpha := newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work"))
	beta := newProject("beta", "Beta", taggedTask("b1", 1, "unrelated", "ops"))
	repo := newFakeRepo(alpha, beta)
	seedRenameCatalog(repo, `{"version":17}`)
	svc := newTestService(repo)
	censusRevisions := map[string]string{"alpha": repo.revisions["alpha"], "beta": repo.revisions["beta"]}

	if _, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	}); err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want exactly one transaction", len(repo.commits))
	}
	changesets := repo.commits[0]
	if len(changesets) != 1 || changesets[0].Project.ID != "alpha" {
		t.Fatalf("changesets = %d, want only the changed project", len(changesets))
	}
	if changesets[0].UndoAction != "Rename Tag" {
		t.Fatalf("undo action = %q, want Rename Tag", changesets[0].UndoAction)
	}
	if !strings.Contains(changesets[0].UndoContext, "work → effort") {
		t.Fatalf("undo context = %q, want it to name both spellings", changesets[0].UndoContext)
	}
	guards := changesets[0].ProjectPreconditions
	if len(guards) != 2 {
		t.Fatalf("project guards = %+v, want every census project guarded", guards)
	}
	for i, projectID := range []string{"alpha", "beta"} {
		if guards[i].ProjectID != projectID || guards[i].ExpectedRevision != censusRevisions[projectID] {
			t.Fatalf("guard %d = %+v, want %s at its census revision", i, guards[i], projectID)
		}
	}
	artifacts := changesets[0].ArtifactPreconditions
	if len(artifacts) != 1 || artifacts[0].Path != fakeCatalogPath || artifacts[0].ExpectedRevision != storage.RawArtifactRevision([]byte(`{"version":17}`)) {
		t.Fatalf("artifact guards = %+v, want the catalog at its census revision", artifacts)
	}
}

// TestRenameTaskNameExpiresWhenAnUnaffectedProjectChanges is the preview
// expiration contract for a project the rename never intended to write: a
// concurrent writer anywhere in the census aborts the whole commit.
func TestRenameTaskNameExpiresWhenAnUnaffectedProjectChanges(t *testing.T) {
	alpha := newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work"))
	beta := newProject("beta", "Beta", taggedTask("b1", 1, "unrelated", "ops"))
	repo := newFakeRepo(alpha, beta)
	svc := newTestService(repo)

	preview, err := svc.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	repo.preCommit = func() { repo.revisions["beta"] = "someone-else-wrote-beta" }

	if _, err := svc.ApplyTaskNameRename(preview); !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("ApplyTaskNameRename error = %v, want ErrStaleRevision", err)
	}
	if got := projectTaskByID(t, repo, "alpha", "a1").Tags; !slices.Equal(got, []string{"work"}) {
		t.Fatalf("alpha tags after refused apply = %v, want the untouched source name", got)
	}
}

// TestRenameTaskNameExpiresWhenAnAffectedProjectChanges is the same contract for
// a project the rename does intend to write. Its own ExpectedRevision would
// invite a three-way merge; the guard is what expires the preview instead, so a
// rename never lands against a document nobody previewed.
func TestRenameTaskNameExpiresWhenAnAffectedProjectChanges(t *testing.T) {
	alpha := newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work"))
	repo := newFakeRepo(alpha)
	svc := newTestService(repo)

	preview, err := svc.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	repo.preCommit = func() { repo.revisions["alpha"] = "someone-else-wrote-alpha" }

	_, err = svc.ApplyTaskNameRename(preview)
	var conflict *storage.RevisionConflictError
	if !errors.As(err, &conflict) || conflict.ProjectID != "alpha" {
		t.Fatalf("ApplyTaskNameRename error = %v, want a RevisionConflictError on the affected project", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("expired apply committed %d transactions, want none", len(repo.commits))
	}
}

// TestRenameTaskNameExpiresWhenTheCatalogChanges covers the other half of the
// read set: the enumeration itself. A catalog written between preview and apply
// means the census may no longer name the right projects, so the rename expires.
func TestRenameTaskNameExpiresWhenTheCatalogChanges(t *testing.T) {
	alpha := newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work"))
	repo := newFakeRepo(alpha)
	seedRenameCatalog(repo, `{"version":17}`)
	svc := newTestService(repo)

	preview, err := svc.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	repo.preCommit = func() { seedRenameCatalog(repo, `{"version":17,"projects":["late"]}`) }

	_, err = svc.ApplyTaskNameRename(preview)
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) || conflict.Path != fakeCatalogPath {
		t.Fatalf("ApplyTaskNameRename error = %v, want an ArtifactConflictError on the catalog", err)
	}
	if got := projectTaskByID(t, repo, "alpha", "a1").Tags; !slices.Equal(got, []string{"work"}) {
		t.Fatalf("alpha tags after refused apply = %v, want the untouched source name", got)
	}
}

// TestRenameTaskNameNoOpCreatesNoTransaction covers the empty case in both
// halves: nothing carries the source name, so the apply neither commits nor
// reports a transaction that could be undone.
func TestRenameTaskNameNoOpCreatesNoTransaction(t *testing.T) {
	repo := newFakeRepo(newProject("alpha", "Alpha", taggedTask("a1", 1, "unrelated", "ops")))
	svc := newTestService(repo)

	result, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if len(result.Tasks) != 0 || len(result.TouchedProjectIDs) != 0 || result.TransactionID != "" {
		t.Fatalf("no-op result = %+v, want the zero value", result)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("no-op committed %d transactions, want none", len(repo.commits))
	}
}

// TestRenameTaskNameFailsClosedOnACensusReadFailure proves the strict half of
// the census contract: one unreadable project fails the whole operation instead
// of renaming across the projects that happened to load.
func TestRenameTaskNameFailsClosedOnACensusReadFailure(t *testing.T) {
	repo := newFakeRepo(newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work")))
	repo.censusErr = fmt.Errorf("reading project Beta for census: broken document")
	svc := newTestService(repo)

	_, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err == nil || !strings.Contains(err.Error(), "broken document") {
		t.Fatalf("RenameTaskName error = %v, want the census read failure", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("failed census committed %d transactions, want none", len(repo.commits))
	}
}

// censuslessRepo is a Repository without the census capability, standing in for
// any adapter that can commit but cannot prove it read the whole workspace.
type censuslessRepo struct{ inner *fakeRepo }

func (r censuslessRepo) Snapshot() (app.Snapshot, error) { return r.inner.Snapshot() }

func (r censuslessRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	return r.inner.Commit(changesets...)
}

// TestRenameTaskNameRefusesARepositoryWithoutACensus keeps the capability a
// refusal by name rather than a silent partial rename over whatever the daily
// snapshot happened to load.
func TestRenameTaskNameRefusesARepositoryWithoutACensus(t *testing.T) {
	inner := newFakeRepo(newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work")))
	svc := app.NewServiceWithArtifacts(censuslessRepo{inner: inner}, fixedClock{t: time.Now()}, &seqIDs{}, nil)

	_, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err == nil || !strings.Contains(err.Error(), "strict workspace census") {
		t.Fatalf("RenameTaskName error = %v, want a refusal naming the missing census", err)
	}
	if len(inner.commits) != 0 {
		t.Fatalf("refused rename committed %d transactions, want none", len(inner.commits))
	}
}

// TestRenameTaskNameNormalizesNamesLikeTheGrammar proves the request is
// canonicalized before anything is scanned, so a request naming "Work" renames
// exactly what "#work" matches and collapses onto the canonical destination.
func TestRenameTaskNameNormalizesNamesLikeTheGrammar(t *testing.T) {
	repo := newFakeRepo(newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work", "effort")))
	svc := newTestService(repo)

	result, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "  Work  ",
		To:        "Effort",
	})
	if err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("renamed %d tasks, want the task the canonical spelling matches", len(result.Tasks))
	}
	if got := projectTaskByID(t, repo, "alpha", "a1").Tags; !slices.Equal(got, []string{"effort"}) {
		t.Fatalf("tags after normalized collision = %v, want one canonical destination", got)
	}
}

// TestRenameTaskNameRejectsInvalidRequests holds every refusal to the validation
// class and proves none of them reaches the repository.
func TestRenameTaskNameRejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name string
		req  app.RenameTaskNameRequest
	}{
		{"unknown namespace", app.RenameTaskNameRequest{Namespace: "label", From: "work", To: "effort"}},
		{"empty namespace", app.RenameTaskNameRequest{From: "work", To: "effort"}},
		{"empty source", app.RenameTaskNameRequest{Namespace: app.TaskTagNamespace, From: "", To: "effort"}},
		{"empty destination", app.RenameTaskNameRequest{Namespace: app.TaskTagNamespace, From: "work", To: ""}},
		{"unmintable destination", app.RenameTaskNameRequest{Namespace: app.TaskTagNamespace, From: "work", To: "!!!"}},
		{"unmintable source", app.RenameTaskNameRequest{Namespace: app.TaskContextNamespace, From: "***", To: "effort"}},
		{"same name after normalization", app.RenameTaskNameRequest{Namespace: app.TaskTagNamespace, From: "Work", To: "work"}},
		{"reserved context source", app.RenameTaskNameRequest{Namespace: app.TaskContextNamespace, From: "0", To: "effort"}},
		{"reserved context destination", app.RenameTaskNameRequest{Namespace: app.TaskContextNamespace, From: "work", To: "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo(newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work")))
			svc := newTestService(repo)
			if _, err := svc.RenameTaskName(tc.req); !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("RenameTaskName error = %v, want ErrInvalidInput", err)
			}
			if repo.censusCalls != 0 || len(repo.commits) != 0 {
				t.Fatalf("invalid request reached the repository: census=%d commits=%d", repo.censusCalls, len(repo.commits))
			}
		})
	}
}

// TestExecuteTaskOperationRoutesTaskNameRename covers the dispatcher entry: the
// one operation whose scope is the workspace rather than its targets.
func TestExecuteTaskOperationRoutesTaskNameRename(t *testing.T) {
	repo := newFakeRepo(newProject("alpha", "Alpha", contextTask("a1", 1, "source", "work")))
	svc := newTestService(repo)

	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationRenameTaskName,
		RenameName: app.RenameTaskNameRequest{
			Namespace: app.TaskContextNamespace,
			From:      "work",
			To:        "effort",
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].TaskID != "a1" {
		t.Fatalf("result tasks = %+v, want the renamed task", result.Tasks)
	}
	if !slices.Equal(result.TouchedProjectIDs, []string{"alpha"}) || result.TransactionID == "" {
		t.Fatalf("result = %+v, want one touched project under one transaction", result)
	}
	if got := projectTaskByID(t, repo, "alpha", "a1").Context; got != "effort" {
		t.Fatalf("context after dispatch = %q, want the renamed value", got)
	}
}
