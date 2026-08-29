package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// childOf seeds a task already parented, which is the state every overlap and
// depth fixture below starts from.
func childOf(id string, shortID int, title string, parent *models.Task) *models.Task {
	task := newTask(id, shortID, title)
	task.ParentID = &parent.ID
	return task
}

// projectTaskIDs reports a project's task order, which is what the batch's
// documented append order is asserted against.
func projectTaskIDs(project *models.Project) []string {
	ids := make([]string, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

// requireOneTransaction proves the whole batch shares one commit call, one undo
// action, and one undo context — which is what makes it one undo entry — and
// returns that call's changesets in commit order.
func requireOneTransaction(t *testing.T, repo *fakeRepo, wantAction string) []storage.ChangeSet {
	t.Helper()
	if len(repo.commits) != 1 {
		t.Fatalf("commit calls = %d, want exactly one transaction", len(repo.commits))
	}
	batch := repo.commits[0]
	if len(batch) == 0 {
		t.Fatal("transaction carried no changeset")
	}
	context := batch[0].UndoContext
	for _, cs := range batch {
		if cs.UndoAction != wantAction || cs.UndoContext != context {
			t.Fatalf("changeset for %s = action %q context %q, want one shared undo entry %q/%q",
				cs.Project.ID, cs.UndoAction, cs.UndoContext, wantAction, context)
		}
	}
	return batch
}

// TestBulkMoveToProjectCollapsesOverlappingSelection covers the one thing a
// batch faces that a single move never does: an ancestor and its own descendant
// both selected. The descendant already travels inside the ancestor's subtree, so
// collapsing to maximal subtrees is what keeps the model from refusing a
// destination that already holds a moved ID.
func TestBulkMoveToProjectCollapsesOverlappingSelection(t *testing.T) {
	root := newTask("root", 1, "Root")
	child := childOf("child", 2, "Child", root)
	grandchild := childOf("grandchild", 3, "Grandchild", child)
	source := newProject("source", "Source", root, child, grandchild)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(source, destination)

	result, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"root", "child", "grandchild"}),
		destination.ID,
	)
	if err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}

	if len(source.Tasks) != 0 {
		t.Fatalf("source retained %v, want the whole subtree gone", projectTaskIDs(source))
	}
	if got := projectTaskIDs(destination); len(got) != 3 {
		t.Fatalf("destination tasks = %v, want the whole subtree", got)
	}
	if destination.GetTaskByID("root") != root || destination.GetTaskByID("grandchild") != grandchild {
		t.Fatalf("destination lost stable task identities: %v", projectTaskIDs(destination))
	}
	if root.ParentID != nil {
		t.Fatalf("moved root parent = %v, want a destination root", root.ParentID)
	}
	if child.ParentID == nil || *child.ParentID != root.ID || grandchild.ParentID == nil || *grandchild.ParentID != child.ID {
		t.Fatalf("subtree shape not preserved: child=%v grandchild=%v", child.ParentID, grandchild.ParentID)
	}
	if root.ShortID != 1 || child.ShortID != 2 || grandchild.ShortID != 3 {
		t.Fatalf("destination short IDs = %d/%d/%d, want a fresh project-local run", root.ShortID, child.ShortID, grandchild.ShortID)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].TaskID != root.ID || result.Tasks[0].ProjectID != destination.ID {
		t.Fatalf("result tasks = %+v, want one entry for the collapsed root at its new address", result.Tasks)
	}
	requireOneTransaction(t, repo, "Move to Project")
}

// TestBulkMoveToProjectMovesEverySourceInOneTransaction covers the N-sources
// case together with the batch's fixed order: sources in first-seen request
// order, and each source's roots in that project's own order regardless of how
// the selection was enumerated.
func TestBulkMoveToProjectMovesEverySourceInOneTransaction(t *testing.T) {
	first := newTask("a-first", 1, "A first")
	second := newTask("a-second", 2, "A second")
	sourceA := newProject("source-a", "Source A", first, second)
	only := newTask("b-only", 1, "B only")
	sourceB := newProject("source-b", "Source B", only)
	resident := newTask("resident", 1, "Resident")
	destination := newProject("destination", "Destination", resident)
	repo := newFakeRepo(sourceA, sourceB, destination)

	// Deliberately scrambled: A is seen first through its *second* task, and B
	// interleaves before A's first task.
	result, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"a-second", "b-only", "a-first"}),
		destination.ID,
	)
	if err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}

	if len(sourceA.Tasks) != 0 || len(sourceB.Tasks) != 0 {
		t.Fatalf("sources retained tasks: A=%v B=%v", projectTaskIDs(sourceA), projectTaskIDs(sourceB))
	}
	want := []string{"resident", "a-first", "a-second", "b-only"}
	if got := projectTaskIDs(destination); !equalIDs(got, want) {
		t.Fatalf("destination order = %v, want %v", got, want)
	}
	if first.ShortID != 2 || second.ShortID != 3 || only.ShortID != 4 {
		t.Fatalf("short IDs = %d/%d/%d, want a continuing destination run", first.ShortID, second.ShortID, only.ShortID)
	}
	wantTouched := []string{"source-a", "source-b", "destination"}
	if !equalIDs(result.TouchedProjectIDs, wantTouched) {
		t.Fatalf("TouchedProjectIDs = %v, want %v", result.TouchedProjectIDs, wantTouched)
	}
	if len(result.Tasks) != 3 {
		t.Fatalf("result tasks = %+v, want one per canonical root", result.Tasks)
	}
	for _, task := range result.Tasks {
		if task.ProjectID != destination.ID || task.TransactionID == "" {
			t.Fatalf("result task = %+v, want the destination address and the shared transaction", task)
		}
	}

	batch := requireOneTransaction(t, repo, "Move to Project")
	if len(batch) != 3 {
		t.Fatalf("changesets = %d, want two sources plus the explicit destination commit", len(batch))
	}
	if batch[0].Project.ID != "source-a" || batch[1].Project.ID != "source-b" || batch[2].Project.ID != "destination" {
		t.Fatalf("commit order = %v, want sources then destination", committedProjectIDs(repo))
	}
}

func equalIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestBulkMoveToProjectCarriesAttachmentMovesPerSource locks the side effect
// that has to ride the same transaction: every moved task's attachment
// directory relocates, and each op is attached to the changeset of the source it
// left rather than being pooled onto one project.
func TestBulkMoveToProjectCarriesAttachmentMovesPerSource(t *testing.T) {
	root := newTask("a-root", 1, "A root")
	child := childOf("a-child", 2, "A child", root)
	sourceA := newProject("source-a", "Source A", root, child)
	only := newTask("b-only", 1, "B only")
	sourceB := newProject("source-b", "Source B", only)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(sourceA, sourceB, destination)

	if _, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"a-root", "b-only"}),
		destination.ID,
	); err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}

	batch := requireOneTransaction(t, repo, "Move to Project")
	wantOps := map[string]int{"source-a": 2, "source-b": 1, "destination": 0}
	for _, cs := range batch {
		if got := len(cs.AttachmentOps); got != wantOps[cs.Project.ID] {
			t.Fatalf("project %s carried %d attachment ops, want %d", cs.Project.ID, got, wantOps[cs.Project.ID])
		}
		for _, op := range cs.AttachmentOps {
			if op.Kind != storage.AttachmentMove || op.SourceProject.ID != cs.Project.ID || op.DestProject.ID != destination.ID {
				t.Fatalf("attachment op = %+v, want a move off %s onto the destination", op, cs.Project.ID)
			}
		}
	}
}

// TestBulkMoveToProjectRequalifiesTodayAndNext proves the durable planning
// memberships follow the batch: Today and Next are project-qualified, so a move
// that changed only the project component would silently orphan both without the
// aggregated requalification.
func TestBulkMoveToProjectRequalifiesTodayAndNext(t *testing.T) {
	today := newTask("a-today", 1, "Planned today")
	sourceA := newProject("source-a", "Source A", today)
	next := newTask("b-next", 1, "Planned next")
	sourceB := newProject("source-b", "Source B", next)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(sourceA, sourceB, destination)

	planning := config.NewPlanningState()
	planning.Today = []config.ProjectTaskLocator{{TaskID: today.ID, ProjectID: sourceA.ID}}
	planning.SetNext([]config.ProjectTaskLocator{{TaskID: next.ID, ProjectID: sourceB.ID}})
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if _, err := svc.BulkMoveToProject(
		repoLocators(repo, []string{"a-today", "b-next"}),
		destination.ID,
	); err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}

	if got := planning.Today; len(got) != 1 || got[0].TaskID != today.ID || got[0].ProjectID != destination.ID {
		t.Fatalf("Today = %+v, want the moved task requalified onto the destination", got)
	}
	if got := planning.GetNext(); len(got) != 1 || got[0].TaskID != next.ID || got[0].ProjectID != destination.ID {
		t.Fatalf("Next = %+v, want the moved task requalified onto the destination", got)
	}

	batch := requireOneTransaction(t, repo, "Move to Project")
	if len(batch[0].Artifacts) != 1 {
		t.Fatalf("first changeset artifacts = %d, want the one planning rewrite riding the batch", len(batch[0].Artifacts))
	}
}

// TestBulkMoveToProjectRewritesReferencesFromArchivedHolder proves the batch
// runs ONE maintenance pass over the whole catalog: a holder outside the
// mutation snapshot — an archived project — is repaired for both moved tasks in
// the same transaction, and it gains exactly one changeset rather than one per
// moved reference.
func TestBulkMoveToProjectRewritesReferencesFromArchivedHolder(t *testing.T) {
	sourceA := newProject("source-a", "Source A", newTask("a-target", 1, "A target"))
	sourceB := newProject("source-b", "Source B", newTask("b-target", 1, "B target"))
	destination := newProject("destination", "Destination")
	archived := newProject("archived", "Archived",
		dependentTask("archived-holder", 1, "Archived holder",
			referenceTo("source-a", "a-target"),
			referenceTo("source-b", "b-target"),
		))
	repo := newFakeRepo(sourceA, sourceB, destination).withUnloaded(archived)

	if _, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"a-target", "b-target"}),
		destination.ID,
	); err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}

	requireReferences(t, archived, "archived-holder",
		referenceTo("destination", "a-target"),
		referenceTo("destination", "b-target"),
	)
	batch := requireOneTransaction(t, repo, "Move to Project")
	seen := 0
	for _, cs := range batch {
		if cs.Project.ID == archived.ID {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("archived holder appeared in %d changesets, want exactly one from a single maintenance pass", seen)
	}
}

// TestBulkMoveToProjectAcceptsMaximalDepthSubtrees pins the batch depth check
// on the boundary it must not falsely reject. Every moved subtree lands at the
// destination root, so a chain already at the nesting cap stays legal; the
// validation runs once over the graph both subtrees produce.
func TestBulkMoveToProjectAcceptsMaximalDepthSubtrees(t *testing.T) {
	chain := make([]*models.Task, 0, models.MaxNestingDepth+1)
	for i := 0; i <= models.MaxNestingDepth; i++ {
		if i == 0 {
			chain = append(chain, newTask("deep-0", 1, "Deep 0"))
			continue
		}
		chain = append(chain, childOf(
			"deep-"+string(rune('0'+i)), i+1, "Deep "+string(rune('0'+i)), chain[i-1],
		))
	}
	sibling := newTask("sibling", 99, "Sibling")
	source := newProject("source", "Source", append(append([]*models.Task(nil), chain...), sibling)...)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(source, destination)

	if got := chain[models.MaxNestingDepth].Level; got != models.MaxNestingDepth {
		t.Fatalf("fixture deepest level = %d, want the nesting cap %d", got, models.MaxNestingDepth)
	}

	if _, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"deep-0", "sibling"}),
		destination.ID,
	); err != nil {
		t.Fatalf("BulkMoveToProject at the depth cap: %v", err)
	}

	if len(destination.Tasks) != models.MaxNestingDepth+2 {
		t.Fatalf("destination tasks = %v, want the whole chain plus the sibling", projectTaskIDs(destination))
	}
	deepest := destination.GetTaskByID(chain[models.MaxNestingDepth].ID)
	if deepest == nil || deepest.Level != models.MaxNestingDepth {
		t.Fatalf("deepest moved task = %+v, want it intact at the cap", deepest)
	}
	requireOneTransaction(t, repo, "Move to Project")
}

// TestBulkMoveToProjectRollsBackWholeBatchOnCommitFailure proves the
// all-or-nothing guarantee at the point it matters: several subtrees have
// already left their sources in memory when the transaction is refused, and
// every one of them must be put back with its identity, short ID, and parent
// edge intact.
func TestBulkMoveToProjectRollsBackWholeBatchOnCommitFailure(t *testing.T) {
	root := newTask("a-root", 1, "A root")
	child := childOf("a-child", 2, "A child", root)
	sourceA := newProject("source-a", "Source A", root, child)
	only := newTask("b-only", 7, "B only")
	sourceB := newProject("source-b", "Source B", only)
	resident := newTask("resident", 1, "Resident")
	destination := newProject("destination", "Destination", resident)
	repo := newFakeRepo(sourceA, sourceB, destination)
	repo.commitErr = errors.New("transaction unavailable")

	_, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"a-root", "b-only"}),
		destination.ID,
	)
	if err == nil {
		t.Fatal("expected the refused transaction to fail the batch")
	}
	if len(repo.commits) != 0 {
		t.Fatalf("failed transaction recorded %d commit(s)", len(repo.commits))
	}
	if got := projectTaskIDs(sourceA); !equalIDs(got, []string{"a-root", "a-child"}) {
		t.Fatalf("source A = %v, want its subtree restored", got)
	}
	if got := projectTaskIDs(sourceB); !equalIDs(got, []string{"b-only"}) {
		t.Fatalf("source B = %v, want its task restored", got)
	}
	if got := projectTaskIDs(destination); !equalIDs(got, []string{"resident"}) {
		t.Fatalf("destination = %v, want no partial batch retained", got)
	}
	if sourceA.GetTaskByID("a-root") != root || sourceB.GetTaskByID("b-only") != only {
		t.Fatal("rollback replaced the live task pointers the destination had been handed")
	}
	if root.ShortID != 1 || child.ShortID != 2 || only.ShortID != 7 {
		t.Fatalf("short IDs = %d/%d/%d, want the source values restored", root.ShortID, child.ShortID, only.ShortID)
	}
	if root.ParentID != nil || child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("parent edges not restored: root=%v child=%v", root.ParentID, child.ParentID)
	}
}

// TestBulkMoveToProjectRollsBackPlanningFailureAfterPartialMoves covers the
// mid-batch refusal that is not the repository's: both subtrees have landed when
// the planning builder refuses, and the batch must unwind exactly as a refused
// commit does.
func TestBulkMoveToProjectRollsBackPlanningFailureAfterPartialMoves(t *testing.T) {
	first := newTask("a-only", 1, "A only")
	sourceA := newProject("source-a", "Source A", first)
	second := newTask("b-only", 1, "B only")
	sourceB := newProject("source-b", "Source B", second)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(sourceA, sourceB, destination)

	wantErr := errors.New("planning unavailable")
	svc := newTestService(repo).WithPlanningArtifacts(func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		return nil, wantErr
	})

	_, err := svc.BulkMoveToProject(repoLocators(repo, []string{"a-only", "b-only"}), destination.ID)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("planning failure recorded %d commit(s)", len(repo.commits))
	}
	if sourceA.GetTaskByID("a-only") != first || sourceB.GetTaskByID("b-only") != second {
		t.Fatalf("sources not restored: A=%v B=%v", projectTaskIDs(sourceA), projectTaskIDs(sourceB))
	}
	if len(destination.Tasks) != 0 {
		t.Fatalf("destination = %v, want no partial batch retained", projectTaskIDs(destination))
	}
}

// TestBulkMoveToProjectTreatsAlreadyHomeTasksAsNoOps matches the single `%`
// move, which routes a same-project pick through an ordinary unchanged edit
// rather than refusing it: a mixed selection moves what is elsewhere, leaves what
// is already home alone, and still reports every root.
func TestBulkMoveToProjectTreatsAlreadyHomeTasksAsNoOps(t *testing.T) {
	mover := newTask("mover", 1, "Mover")
	source := newProject("source", "Source", mover)
	resident := newTask("resident", 1, "Resident")
	destination := newProject("destination", "Destination", resident)
	repo := newFakeRepo(source, destination)

	result, err := newTestService(repo).BulkMoveToProject(
		repoLocators(repo, []string{"mover", "resident"}),
		destination.ID,
	)
	if err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}
	if got := projectTaskIDs(destination); !equalIDs(got, []string{"resident", "mover"}) {
		t.Fatalf("destination = %v, want the resident untouched and the mover appended", got)
	}
	if resident.ShortID != 1 {
		t.Fatalf("resident short ID = %d, want it left alone", resident.ShortID)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("result tasks = %+v, want one entry per selected root", result.Tasks)
	}
	batch := requireOneTransaction(t, repo, "Move to Project")
	if len(batch) != 2 {
		t.Fatalf("changesets = %d, want the one source plus the destination", len(batch))
	}
}

// TestBulkMoveToProjectAllHomeSelectionCommitsUnchanged is the degenerate case
// of the same rule: nothing travels, so nothing is asked of the planning builder,
// and the destination still commits the way every idempotent bulk mutation does.
func TestBulkMoveToProjectAllHomeSelectionCommitsUnchanged(t *testing.T) {
	resident := newTask("resident", 1, "Resident")
	destination := newProject("destination", "Destination", resident)
	repo := newFakeRepo(destination)
	built := 0
	svc := newTestService(repo).WithPlanningArtifacts(func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
		built++
		return nil, nil
	})

	result, err := svc.BulkMoveToProject(repoLocators(repo, []string{"resident"}), destination.ID)
	if err != nil {
		t.Fatalf("BulkMoveToProject: %v", err)
	}
	if built != 0 {
		t.Fatalf("planning builder ran %d time(s), want none for a batch that moved nothing", built)
	}
	if !equalIDs(result.TouchedProjectIDs, []string{"destination"}) {
		t.Fatalf("TouchedProjectIDs = %v, want only the destination", result.TouchedProjectIDs)
	}
	requireOneTransaction(t, repo, "Move to Project")
}

// TestBulkMoveToProjectRejectsMalformedRequests keeps the batch's refusals
// ahead of every mutation: an empty selection, a missing destination, and an
// unknown destination all abort before a snapshot is touched.
func TestBulkMoveToProjectRejectsMalformedRequests(t *testing.T) {
	task := newTask("task", 1, "Task")
	source := newProject("source", "Source", task)
	destination := newProject("destination", "Destination")
	repo := newFakeRepo(source, destination)
	svc := newTestService(repo)

	if _, err := svc.BulkMoveToProject(nil, destination.ID); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty selection error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.BulkMoveToProject(repoLocators(repo, []string{"task"}), "  "); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("blank destination error = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.BulkMoveToProject(repoLocators(repo, []string{"task"}), "missing"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("unknown destination error = %v, want ErrProjectNotFound", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("refused requests recorded %d commit(s)", len(repo.commits))
	}
	if source.GetTaskByID("task") != task || len(destination.Tasks) != 0 {
		t.Fatal("a refused request mutated the workspace")
	}
}
