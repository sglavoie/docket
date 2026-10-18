package app_test

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// promoteFrozen issues the promotion the UI issues: the complete ordered
// checklist the caller just read, frozen by value, and the index inside it. The
// copy matters — a test that handed the service the owner's own backing slice
// would compare the list against itself and never exercise the freeze.
func promoteFrozen(
	svc *app.Service, locator app.TaskLocator, owner *models.Task, index int,
) (app.ChecklistPromotionResult, error) {
	return svc.PromoteChecklistItem(app.ChecklistPromotionRequest{
		Owner:     locator,
		Index:     index,
		Checklist: append([]models.ChecklistItem(nil), owner.Checklist...),
	})
}

// promotionFixtureChecklist is the seed every conversion case shares. It holds a
// literal capture sigil, a literal destination sigil, one text that appears twice
// with different completion flags, and a checked item, so one fixture answers the
// literal-text, duplicate-text, and checked/unchecked questions at once.
func promotionFixtureChecklist() []models.ChecklistItem {
	return []models.ChecklistItem{
		{Text: "#auth rewrite"},
		{Text: "duplicate"},
		{Text: "duplicate", Done: true},
		{Text: "$done tomorrow"},
	}
}

// newPromotionFixture seeds one project holding one owner task with
// promotionFixtureChecklist, returning the repo, the service, and the owner.
func newPromotionFixture(t *testing.T) (*fakeRepo, *app.Service, *models.Task) {
	t.Helper()
	owner := newTask("owner", 1, "Owner")
	owner.Checklist = promotionFixtureChecklist()
	repo := newFakeRepo(newProject("p1", "Alpha", owner))
	return repo, newTestService(repo), owner
}

// TestPromoteChecklistItemConvertsExactlyTheNamedItem covers the first, a middle,
// and the last position of one checklist, including the duplicated text and the
// checked entry. The promoted title is the item's literal text — a `#tag` or a
// `$state` inside it is prose, because nothing reparses it — and the remaining
// items keep their original order and completion flags.
func TestPromoteChecklistItemConvertsExactlyTheNamedItem(t *testing.T) {
	cases := []struct {
		name      string
		index     int
		wantTitle string
		wantState models.WorkflowState
		wantLeft  []models.ChecklistItem
	}{
		{
			name:      "first item keeps its capture sigil literal",
			index:     0,
			wantTitle: "#auth rewrite",
			wantState: models.StateBacklog,
			wantLeft: []models.ChecklistItem{
				{Text: "duplicate"}, {Text: "duplicate", Done: true}, {Text: "$done tomorrow"},
			},
		},
		{
			name:      "middle item takes the unchecked twin",
			index:     1,
			wantTitle: "duplicate",
			wantState: models.StateBacklog,
			wantLeft: []models.ChecklistItem{
				{Text: "#auth rewrite"}, {Text: "duplicate", Done: true}, {Text: "$done tomorrow"},
			},
		},
		{
			name:      "checked twin becomes a completed task",
			index:     2,
			wantTitle: "duplicate",
			wantState: models.StateDone,
			wantLeft: []models.ChecklistItem{
				{Text: "#auth rewrite"}, {Text: "duplicate"}, {Text: "$done tomorrow"},
			},
		},
		{
			name:      "last item keeps its destination sigil literal",
			index:     3,
			wantTitle: "$done tomorrow",
			wantState: models.StateBacklog,
			wantLeft: []models.ChecklistItem{
				{Text: "#auth rewrite"}, {Text: "duplicate"}, {Text: "duplicate", Done: true},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, svc, owner := newPromotionFixture(t)
			res, err := promoteFrozen(svc, repo.loc("owner"), owner, tc.index)
			if err != nil {
				t.Fatalf("PromoteChecklistItem: %v", err)
			}

			if !slicesEqualChecklist(owner.Checklist, tc.wantLeft) {
				t.Fatalf("remaining checklist = %+v, want %+v", owner.Checklist, tc.wantLeft)
			}
			child := repo.projects[0].GetTaskByID(res.Child.TaskID)
			if child == nil {
				t.Fatalf("committed child %q missing from the project", res.Child.TaskID)
			}
			if child.Title != tc.wantTitle {
				t.Fatalf("child title = %q, want the item's literal text %q", child.Title, tc.wantTitle)
			}
			if child.State() != tc.wantState {
				t.Fatalf("child state = %s, want %s", child.State(), tc.wantState)
			}
			if child.ParentID == nil || *child.ParentID != owner.ID {
				t.Fatalf("child parent = %v, want the owner %q", child.ParentID, owner.ID)
			}
			if child.ChecklistOrigin == nil || child.ChecklistOrigin.ProjectID != "p1" ||
				child.ChecklistOrigin.TaskID != owner.ID || child.ChecklistOrigin.Index != tc.index {
				t.Fatalf("child origin = %+v, want owner and item index %d", child.ChecklistOrigin, tc.index)
			}
			if len(repo.commits) != 1 {
				t.Fatalf("commits = %d, want one transaction covering both halves", len(repo.commits))
			}
			if res.Owner.TransactionID == "" || res.Owner.TransactionID != res.Child.TransactionID {
				t.Fatalf("owner txn %q and child txn %q must be the one committed transaction",
					res.Owner.TransactionID, res.Child.TransactionID)
			}
			if res.Owner.Revision == "" || res.Owner.Revision != res.Child.Revision {
				t.Fatalf("owner revision %q and child revision %q must match",
					res.Owner.Revision, res.Child.Revision)
			}
			if res.Child.TaskID == "" || res.Child.ShortID == 0 {
				t.Fatalf("child identity = %+v, want a committed id the UI can select", res.Child)
			}
		})
	}
}

// TestPromotedChildCopiesNoOwnerMetadata holds the child to the one thing a
// checklist item carries: its text and its completion flag. Everything the owner
// happens to store stays with the owner.
func TestPromotedChildCopiesNoOwnerMetadata(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	owner.Priority = 1
	owner.Tags = []string{"infra"}
	owner.Context = "office"
	owner.Description = "owner description"
	owner.Pinned = true
	owner.Review = true
	owner.Checklist = []models.ChecklistItem{{Text: "Write the adapter"}}
	repo := newFakeRepo(newProject("p1", "Alpha", owner))
	svc := newTestService(repo)

	res, err := promoteFrozen(svc, repo.loc("owner"), owner, 0)
	if err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}
	child := repo.projects[0].GetTaskByID(res.Child.TaskID)
	switch {
	case child.Priority != models.PriorityUnset:
		t.Fatalf("child priority = %v, want unset", child.Priority)
	case len(child.Tags) != 0:
		t.Fatalf("child tags = %v, want none", child.Tags)
	case child.Context != "":
		t.Fatalf("child context = %q, want empty", child.Context)
	case child.Description != "":
		t.Fatalf("child description = %q, want empty", child.Description)
	case child.Pinned:
		t.Fatal("child must not inherit the owner's pin")
	case child.Review:
		t.Fatal("child must not inherit the owner's review flag")
	case child.DueDate != nil || child.DeferredUntil != nil:
		t.Fatalf("child dates = %v/%v, want none", child.DueDate, child.DeferredUntil)
	case len(child.Checklist) != 0:
		t.Fatalf("child checklist = %+v, want none", child.Checklist)
	case len(child.Comments) != 0 || len(child.Attachments) != 0 || len(child.Dependencies) != 0:
		t.Fatal("child must carry no comments, attachments, or references")
	}
	if child.CompletedAt != nil {
		t.Fatalf("unchecked item produced CompletedAt %v, want nil", child.CompletedAt)
	}
}

// TestPromotedCheckedItemRecordsCompletionAtConversion proves the checked half of
// the destination rule goes through the one lifecycle initialization path: the
// completion instant is the service clock at conversion, not the item's age.
func TestPromotedCheckedItemRecordsCompletionAtConversion(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	owner.Checklist = []models.ChecklistItem{{Text: "Ship it", Done: true}}
	repo := newFakeRepo(newProject("p1", "Alpha", owner))
	svc := newTestService(repo)

	res, err := promoteFrozen(svc, repo.loc("owner"), owner, 0)
	if err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}
	child := repo.projects[0].GetTaskByID(res.Child.TaskID)
	if child.State() != models.StateDone {
		t.Fatalf("child state = %s, want done", child.State())
	}
	if child.CompletedAt == nil {
		t.Fatal("checked item produced no completion instant")
	}
	if !child.CompletedAt.Equal(child.StateChangedAt) {
		t.Fatalf("CompletedAt %v and StateChangedAt %v must be the one transition instant",
			child.CompletedAt, child.StateChangedAt)
	}
}

// TestPromoteChecklistItemAppendsAfterExistingChildren proves the promotion reuses
// the ordinary append ordering rather than inventing one: an owner that already
// has children gains the new task at the end, with the project's next short ID.
func TestPromoteChecklistItemAppendsAfterExistingChildren(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	owner.Checklist = []models.ChecklistItem{{Text: "Third thing"}}
	existing := newTask("existing", 2, "Already a subtask")
	ownerID := owner.ID
	existing.ParentID = &ownerID
	project := newProject("p1", "Alpha", owner, existing)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := promoteFrozen(svc, repo.loc("owner"), owner, 0)
	if err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}
	if res.Child.ShortID != 3 {
		t.Fatalf("child short id = %d, want the project's next free number 3", res.Child.ShortID)
	}
	if len(owner.Children) != 2 {
		t.Fatalf("owner children = %d, want the existing child plus the promoted one", len(owner.Children))
	}
	if owner.Children[0].ID != existing.ID || owner.Children[1].ID != res.Child.TaskID {
		t.Fatalf("child order = %q,%q, want the promotion appended last",
			owner.Children[0].ID, owner.Children[1].ID)
	}
}

// TestPromoteChecklistItemReopensCompletedAncestors proves the shared
// post-mutation reconciliation runs inside the one transaction: promoting an
// unchecked item under a completed owner reopens the owner and every completed
// ancestor above it, because a done task may hold no open child.
func TestPromoteChecklistItemReopensCompletedAncestors(t *testing.T) {
	grandparent := newTask("grandparent", 1, "Epic")
	owner := newTask("owner", 2, "Story")
	grandparentID := grandparent.ID
	owner.ParentID = &grandparentID
	owner.Checklist = []models.ChecklistItem{{Text: "One more thing"}}
	grandparent.Complete()
	owner.Complete()
	repo := newFakeRepo(newProject("p1", "Alpha", grandparent, owner))
	svc := newTestService(repo)

	if _, err := promoteFrozen(svc, repo.loc("owner"), owner, 0); err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}
	if owner.State() != models.StateDoing || owner.CompletedAt != nil {
		t.Fatalf("owner = %s/%v, want reopened with no completion instant", owner.State(), owner.CompletedAt)
	}
	if grandparent.State() != models.StateDoing || grandparent.CompletedAt != nil {
		t.Fatalf("grandparent = %s/%v, want reopened to the fixpoint",
			grandparent.State(), grandparent.CompletedAt)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want the reconciliation inside the one transaction", len(repo.commits))
	}
}

// TestPromoteChecklistItemUndoRedoAcceptance drives the promotion through the real
// transactional commit service with the workspace ledger attached. It is the
// durable proof that one promotion is one ledger entry: undo restores the item at
// its original position and removes the child, and redo brings back the same
// child identity rather than minting a new one.
func TestPromoteChecklistItemUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(
		filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Owner")
	owner := project.Tasks[0]
	seeded := []models.ChecklistItem{
		{Text: "first"}, {Text: "second", Done: true}, {Text: "third"},
	}
	owner.Checklist = append([]models.ChecklistItem(nil), seeded...)

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	appSvc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	res, err := appSvc.PromoteChecklistItem(app.ChecklistPromotionRequest{
		Owner:     repo.loc(owner.ID),
		Index:     1,
		Checklist: append([]models.ChecklistItem(nil), seeded...),
	})
	if err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}

	after := reloadPromotionProject(t, dataDir, project)
	promoted := after.GetTaskByID(res.Child.TaskID)
	if promoted == nil {
		t.Fatalf("child %q absent from tasks.md", res.Child.TaskID)
	}
	if promoted.Title != "second" || promoted.State() != models.StateDone {
		t.Fatalf("durable child = %q/%s, want the checked item as a done task",
			promoted.Title, promoted.State())
	}
	if promoted.ParentID == nil || *promoted.ParentID != owner.ID {
		t.Fatalf("durable child parent = %v, want the owner", promoted.ParentID)
	}
	if promoted.ChecklistOrigin == nil || promoted.ChecklistOrigin.ProjectID != project.ID ||
		promoted.ChecklistOrigin.TaskID != owner.ID || promoted.ChecklistOrigin.Index != 1 {
		t.Fatalf("durable child origin = %+v", promoted.ChecklistOrigin)
	}
	if left := after.GetTaskByID(owner.ID).Checklist; !slicesEqualChecklist(
		left, []models.ChecklistItem{{Text: "first"}, {Text: "third"}}) {
		t.Fatalf("durable checklist = %+v, want the item consumed", left)
	}

	// Undo takes both halves back through the one ledger entry.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	undone := reloadPromotionProject(t, dataDir, project)
	if undone.GetTaskByID(res.Child.TaskID) != nil {
		t.Fatal("undo left the promoted child behind")
	}
	if left := undone.GetTaskByID(owner.ID).Checklist; !slicesEqualChecklist(left, seeded) {
		t.Fatalf("undone checklist = %+v, want the item back at its original position", left)
	}

	// Redo restores the same identity: the child the UI was told about is the child
	// that comes back, short ID included.
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	redone := reloadPromotionProject(t, dataDir, project)
	restored := redone.GetTaskByID(res.Child.TaskID)
	if restored == nil {
		t.Fatalf("redo minted a different child than %q", res.Child.TaskID)
	}
	if restored.ShortID != res.Child.ShortID {
		t.Fatalf("redone short id = %d, want %d", restored.ShortID, res.Child.ShortID)
	}
	if left := redone.GetTaskByID(owner.ID).Checklist; !slicesEqualChecklist(
		left, []models.ChecklistItem{{Text: "first"}, {Text: "third"}}) {
		t.Fatalf("redone checklist = %+v, want the item consumed again", left)
	}
}

// reloadPromotionProject reads a seeded project back from disk, so an assertion
// reads what the transaction durably wrote rather than the in-memory objects the
// use case mutated.
func reloadPromotionProject(t *testing.T, dataDir string, project *models.Project) *models.Project {
	t.Helper()
	reloaded := &models.Project{
		ID:   project.ID,
		Name: project.Name,
		Slug: project.Slug,
		Path: filepath.Join(dataDir, "projects", project.Slug),
	}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	return reloaded
}

// requirePromotionChangedNothing asserts a refused promotion opened no
// transaction at all, which is the whole-or-nothing half of the contract the
// rollback assertions complete.
func requirePromotionChangedNothing(t *testing.T, repo *fakeRepo) {
	t.Helper()
	if len(repo.commits) != 0 {
		t.Fatalf("refused promotion recorded %d commits, want none", len(repo.commits))
	}
}

// slicesEqualChecklist compares two checklists by value, flags included, so a
// duplicate text that differs only in completion is not mistaken for a match.
func slicesEqualChecklist(got, want []models.ChecklistItem) bool {
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
