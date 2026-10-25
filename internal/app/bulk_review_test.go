package app_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// toggleReview runs the review toggle through the one application dispatcher, so
// every test below exercises the path a presentation actually takes rather than
// the bulk primitive underneath it.
func toggleReview(svc *app.Service, targets ...app.TaskLocator) (app.TaskOperationResult, error) {
	return svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationToggleReview,
		Targets:   targets,
	})
}

func TestToggleReviewFlagsUntilAllFlaggedThenClears(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	project := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// One target: the single cardinality is the same decision over a selection of
	// one, flagging an unflagged task and clearing it again.
	if _, err := toggleReview(svc, repo.loc("t1")); err != nil {
		t.Fatalf("single toggle: %v", err)
	}
	if !first.Review || second.Review {
		t.Fatalf("single toggle review = %t/%t, want true/false", first.Review, second.Review)
	}
	if _, err := toggleReview(svc, repo.loc("t1")); err != nil {
		t.Fatalf("single untoggle: %v", err)
	}
	if first.Review {
		t.Fatal("second single toggle left the task flagged")
	}

	// All clear: every member is flagged.
	result, err := toggleReview(svc, repo.loc("t1"), repo.loc("t2"))
	if err != nil {
		t.Fatalf("all-clear toggle: %v", err)
	}
	if !first.Review || !second.Review {
		t.Fatalf("all-clear toggle review = %t/%t, want true/true", first.Review, second.Review)
	}
	if len(result.Tasks) != 2 || len(result.TouchedProjectIDs) != 1 || result.TransactionID == "" {
		t.Fatalf("all-clear result = %+v, want two task results carrying one project and a transaction", result)
	}

	// All set: only a fully flagged selection clears.
	if _, err := toggleReview(svc, repo.loc("t1"), repo.loc("t2")); err != nil {
		t.Fatalf("all-set toggle: %v", err)
	}
	if first.Review || second.Review {
		t.Fatalf("all-set toggle review = %t/%t, want false/false", first.Review, second.Review)
	}
}

func TestToggleReviewFlagsMixedSelectionAndPreservesUnchangedMemberTimestamps(t *testing.T) {
	flagged := newTask("flagged", 1, "already flagged")
	flagged.Review = true
	stale := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	flagged.UpdatedAt = stale
	clear := newTask("clear", 2, "not flagged")
	clear.UpdatedAt = stale
	project := newProject("proj-a", "Alpha", flagged, clear)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := toggleReview(svc, repo.loc("flagged"), repo.loc("clear")); err != nil {
		t.Fatalf("mixed toggle: %v", err)
	}
	if !flagged.Review || !clear.Review {
		t.Fatalf("mixed toggle review = %t/%t, want true/true", flagged.Review, clear.Review)
	}
	// The already-correct member is a no-op, so its touch time still answers "when
	// did this task last change" rather than "when was the selection toggled".
	if !flagged.UpdatedAt.Equal(stale) {
		t.Errorf("unchanged member UpdatedAt = %v, want the untouched %v", flagged.UpdatedAt, stale)
	}
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if !clear.UpdatedAt.Equal(wantNow) {
		t.Errorf("changed member UpdatedAt = %v, want the injected clock %v", clear.UpdatedAt, wantNow)
	}
}

// TestToggleReviewStampsOneOperationTimestamp proves the clock is read once for
// the whole selection: an advancing clock would hand each mutated member a
// different instant if the toggle read it per target.
func TestToggleReviewStampsOneOperationTimestamp(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	project := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(project)
	clock := &advancingClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), step: time.Minute}
	svc := newTestServiceWithClock(repo, clock)

	if _, err := toggleReview(svc, repo.loc("t1"), repo.loc("t2")); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !first.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("member timestamps = %v/%v, want one operation instant", first.UpdatedAt, second.UpdatedAt)
	}
}

func TestToggleReviewDeduplicatesLocatorsWithoutConflatingProjects(t *testing.T) {
	inA := newTask("shared", 1, "in alpha")
	inB := newTask("shared", 1, "in beta")
	projectA := newProject("proj-a", "Alpha", inA)
	projectB := newProject("proj-b", "Beta", inB)
	repo := newFakeRepo(projectA, projectB)
	svc := newTestService(repo)

	target := app.TaskLocator{ProjectID: "proj-a", TaskID: "shared"}
	result, err := toggleReview(svc, target, target, target)
	if err != nil {
		t.Fatalf("duplicate toggle: %v", err)
	}
	// A duplicate collapses rather than toggling twice, and the identically named
	// task in the other project is a different task, not a repeat of this one.
	if !inA.Review {
		t.Fatal("duplicated locators canceled themselves out")
	}
	if inB.Review {
		t.Fatal("matching task id in another project was flagged")
	}
	if len(result.Tasks) != 1 || len(result.TouchedProjectIDs) != 1 {
		t.Fatalf("duplicate result = %+v, want one task in one project", result)
	}

	// Both identities together are one selection of two distinct tasks: neither is
	// flagged yet in Beta, so the whole selection flags.
	if _, err := toggleReview(svc, target, app.TaskLocator{ProjectID: "proj-b", TaskID: "shared"}); err != nil {
		t.Fatalf("cross-project toggle: %v", err)
	}
	if !inA.Review || !inB.Review {
		t.Fatalf("cross-project review = %t/%t, want true/true", inA.Review, inB.Review)
	}
}

func TestToggleReviewRejectsUnusableTargetSetsBeforeMutation(t *testing.T) {
	task := newTask("t1", 1, "first")
	project := newProject("proj-a", "Alpha", task)
	archivedTask := newTask("archived-task", 1, "out of the read model")
	archived := newProject("proj-archived", "Archived", archivedTask)
	repo := newFakeRepo(project).withUnloaded(archived)
	svc := newTestService(repo)

	if _, err := toggleReview(svc); !errors.Is(err, app.ErrInvalidInput) {
		t.Errorf("empty target set err = %v, want ErrInvalidInput", err)
	}
	for _, malformed := range []app.TaskLocator{
		{ProjectID: "", TaskID: "t1"},
		{ProjectID: "proj-a", TaskID: ""},
		{ProjectID: "proj-a", TaskID: "   "},
	} {
		if _, err := toggleReview(svc, repo.loc("t1"), malformed); !errors.Is(err, app.ErrInvalidInput) {
			t.Errorf("malformed target %+v err = %v, want ErrInvalidInput", malformed, err)
		}
	}
	if _, err := toggleReview(svc, repo.loc("t1"), app.TaskLocator{ProjectID: "proj-a", TaskID: "missing"}); !errors.Is(err, app.ErrTaskNotFound) {
		t.Errorf("missing target err = %v, want ErrTaskNotFound", err)
	}
	// An archived project is outside the mutation read model, so a target inside one
	// is unwritable and fails the same way a missing task does.
	if _, err := toggleReview(svc, repo.loc("t1"), app.TaskLocator{ProjectID: "proj-archived", TaskID: "archived-task"}); !errors.Is(err, app.ErrTaskNotFound) {
		t.Errorf("archived target err = %v, want ErrTaskNotFound", err)
	}
	if task.Review || archivedTask.Review {
		t.Errorf("refused batch mutated a task: t1=%t archived=%t", task.Review, archivedTask.Review)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (every refusal precedes mutation)", len(repo.commits))
	}
}

func TestToggleReviewCommitsAffectedProjectsInOneTransaction(t *testing.T) {
	inA := newTask("t1", 1, "in alpha")
	inB := newTask("t2", 1, "in beta")
	projectA := newProject("proj-a", "Alpha", inA)
	projectB := newProject("proj-b", "Beta", inB)
	repo := newFakeRepo(projectA, projectB)
	svc := newTestService(repo)

	result, err := toggleReview(svc, repo.loc("t1"), repo.loc("t2"))
	if err != nil {
		t.Fatalf("cross-project toggle: %v", err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (both projects in one transaction)", len(repo.commits))
	}
	if len(repo.commits[0]) != 2 {
		t.Fatalf("changesets in the transaction = %d, want 2", len(repo.commits[0]))
	}
	for _, cs := range repo.commits[0] {
		if cs.UndoAction != "Toggle Review" || cs.UndoContext != "review" {
			t.Errorf("undo entry = %q/%q, want Toggle Review/review", cs.UndoAction, cs.UndoContext)
		}
		// Review is a stored flag beside Pinned, not a planning act: nothing outside
		// the task documents is rewritten.
		if len(cs.Artifacts) != 0 {
			t.Errorf("changeset carried %d artifacts, want 0", len(cs.Artifacts))
		}
	}
	wantTouched := map[string]bool{"proj-a": true, "proj-b": true}
	for _, id := range result.TouchedProjectIDs {
		delete(wantTouched, id)
	}
	if len(wantTouched) != 0 {
		t.Errorf("TouchedProjectIDs = %v, want proj-a and proj-b", result.TouchedProjectIDs)
	}
	if len(result.Tasks) != 2 {
		t.Errorf("result tasks = %d, want 2", len(result.Tasks))
	}
}

func TestToggleReviewRollsBackAnUnwritableSelection(t *testing.T) {
	flagged := newTask("flagged", 1, "already flagged")
	flagged.Review = true
	clear := newTask("clear", 2, "not flagged")
	projectA := newProject("proj-a", "Alpha", flagged, clear)
	inB := newTask("t3", 1, "in beta")
	projectB := newProject("proj-b", "Beta", inB)
	repo := newFakeRepo(projectA, projectB)
	svc := newTestService(repo)

	repo.commitErr = errors.New("commit unavailable")
	_, err := toggleReview(svc, repo.loc("flagged"), repo.loc("clear"), repo.loc("t3"))
	if err == nil {
		t.Fatal("failed review commit returned nil error")
	}
	if !flagged.Review || clear.Review || inB.Review {
		t.Fatalf("rollback left review = %t/%t/%t, want true/false/false", flagged.Review, clear.Review, inB.Review)
	}
}

func TestToggleReviewTouchesOnlyItsTargetsAndNoOtherAxis(t *testing.T) {
	parent := newTask("parent", 1, "parent")
	child := childOf("child", 2, "child", parent)
	sibling := newTask("sibling", 3, "sibling")
	project := newProject("proj-a", "Alpha", parent, child, sibling)

	stateClock := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	if err := parent.ApplyStateTransitionAt(models.StateWaiting, stateClock); err != nil {
		t.Fatalf("seed waiting state: %v", err)
	}
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	parent.DueDate = &due
	deferred := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	parent.DeferredUntil = &deferred
	parent.Pinned = true
	pinnedAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	parent.PinnedAt = &pinnedAt

	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := toggleReview(svc, repo.loc("parent")); err != nil {
		t.Fatalf("toggle parent: %v", err)
	}
	if !parent.Review {
		t.Fatal("targeted parent was not flagged")
	}
	// Parentage is not participation: neither the child nor an unrelated sibling is
	// implicitly carried into the operation.
	if child.Review || sibling.Review {
		t.Fatalf("untargeted tasks flagged: child=%t sibling=%t", child.Review, sibling.Review)
	}
	if parent.State() != models.StateWaiting || !parent.StateChangedAt.Equal(stateClock) {
		t.Errorf("review write moved the lifecycle state or its clock: %s at %v", parent.State(), parent.StateChangedAt)
	}
	if parent.DueDate == nil || !parent.DueDate.Equal(due) {
		t.Errorf("due date = %v, want %v", parent.DueDate, due)
	}
	if parent.DeferredUntil == nil || !parent.DeferredUntil.Equal(deferred) {
		t.Errorf("deferral = %v, want %v", parent.DeferredUntil, deferred)
	}
	if !parent.Pinned || parent.PinnedAt == nil || !parent.PinnedAt.Equal(pinnedAt) {
		t.Errorf("pin = %t at %v, want the untouched pin", parent.Pinned, parent.PinnedAt)
	}

	// The child toggles on its own terms, and its parent stays where the previous
	// operation left it.
	if _, err := toggleReview(svc, repo.loc("child")); err != nil {
		t.Fatalf("toggle child: %v", err)
	}
	if !child.Review || !parent.Review {
		t.Fatalf("independent child toggle = %t with parent %t, want true/true", child.Review, parent.Review)
	}
}

// TestToggleReviewCompletedTargetKeepsCompletion proves the flag coexists with a
// terminal lifecycle state instead of refusing or reopening it: review is a
// stored axis beside the state, never a state of its own.
func TestToggleReviewCompletedTargetKeepsCompletion(t *testing.T) {
	done := newTask("done", 1, "shipped")
	shippedAt := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	if err := done.ApplyStateTransitionAt(models.StateDone, shippedAt); err != nil {
		t.Fatalf("seed done state: %v", err)
	}
	project := newProject("proj-a", "Alpha", done)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	completedAt := done.CompletedAt
	if completedAt == nil {
		t.Fatal("fixture did not record a completion instant")
	}
	if _, err := toggleReview(svc, repo.loc("done")); err != nil {
		t.Fatalf("toggle done task: %v", err)
	}
	if !done.Review || done.State() != models.StateDone {
		t.Fatalf("review = %t with state %s, want true/done", done.Review, done.State())
	}
	if done.CompletedAt == nil || !done.CompletedAt.Equal(*completedAt) {
		t.Errorf("completion instant = %v, want the untouched %v", done.CompletedAt, *completedAt)
	}
}

// TestToggleReviewUndoRedoAcceptance drives the operation through the real
// transactional commit service, so the flag written by one toggle is proven
// reversible and re-appliable through the workspace ledger — one entry for the
// whole selection.
func TestToggleReviewUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	alpha := realTestProject(t, dataDir, "alpha", "review me")
	beta := realTestProject(t, dataDir, "beta", "review me too")
	repo := newStorageRepo(commitSvc, alpha, beta)
	repo.seed(t)

	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	if _, err := toggleReview(svc, repo.loc(alpha.Tasks[0].ID), repo.loc(beta.Tasks[0].ID)); err != nil {
		t.Fatalf("toggle review: %v", err)
	}
	requireStoredReview(t, alpha, true)
	requireStoredReview(t, beta, true)

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	requireStoredReview(t, alpha, false)
	requireStoredReview(t, beta, false)

	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	requireStoredReview(t, alpha, true)
	requireStoredReview(t, beta, true)
}

// requireStoredReview reads the project's committed document back off disk, so an
// assertion answers what was persisted rather than what the in-memory task still
// carries.
func requireStoredReview(t *testing.T, project *models.Project, want bool) {
	t.Helper()
	doc, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("load %s document: %v", project.Slug, err)
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("%s document holds %d tasks, want 1", project.Slug, len(doc.Tasks))
	}
	if doc.Tasks[0].Review != want {
		t.Fatalf("%s stored review = %t, want %t", project.Slug, doc.Tasks[0].Review, want)
	}
}
