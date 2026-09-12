package app_test

import (
	"errors"
	"path/filepath"
	"strings"
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

// TestPromoteChecklistItemRefusesChangedChecklist is the item's identity check. A
// checklist item stores no ID, so the frozen list is what names one: a reorder, a
// tick, an insertion, or a removal between read and gesture refuses the promotion
// whole rather than consuming whichever item drifted under the index.
func TestPromoteChecklistItemRefusesChangedChecklist(t *testing.T) {
	cases := []struct {
		name   string
		frozen []models.ChecklistItem
	}{
		{
			name: "an item was ticked",
			frozen: []models.ChecklistItem{
				{Text: "#auth rewrite"}, {Text: "duplicate"},
				{Text: "duplicate"}, {Text: "$done tomorrow"},
			},
		},
		{
			name: "two items were swapped",
			frozen: []models.ChecklistItem{
				{Text: "duplicate"}, {Text: "#auth rewrite"},
				{Text: "duplicate", Done: true}, {Text: "$done tomorrow"},
			},
		},
		{
			name: "an item was removed",
			frozen: []models.ChecklistItem{
				{Text: "#auth rewrite"}, {Text: "duplicate"}, {Text: "duplicate", Done: true},
			},
		},
		{
			name:   "an item was added",
			frozen: append(promotionFixtureChecklist(), models.ChecklistItem{Text: "late arrival"}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, svc, owner := newPromotionFixture(t)
			_, err := svc.PromoteChecklistItem(app.ChecklistPromotionRequest{
				Owner: repo.loc("owner"), Index: 1, Checklist: tc.frozen,
			})
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if !slicesEqualChecklist(owner.Checklist, promotionFixtureChecklist()) {
				t.Fatalf("refused promotion mutated the checklist: %+v", owner.Checklist)
			}
			requirePromotionChangedNothing(t, repo)
		})
	}
}

// TestPromoteChecklistItemRefusesIndexOutsideChecklist keeps the ordinary range
// error: the index is validated against the frozen list the owner still carries.
func TestPromoteChecklistItemRefusesIndexOutsideChecklist(t *testing.T) {
	for _, index := range []int{-1, 4} {
		repo, svc, owner := newPromotionFixture(t)
		_, err := promoteFrozen(svc, repo.loc("owner"), owner, index)
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("index %d err = %v, want ErrInvalidInput", index, err)
		}
		requirePromotionChangedNothing(t, repo)
	}
}

// TestPromoteChecklistItemRefusesTextNoTaskTitleCanHold proves the literal text is
// held to every rule a stored title obeys, before the item is consumed. The text
// is never rewritten to fit: a promotion that cannot preserve what the item says
// changes nothing at all.
func TestPromoteChecklistItemRefusesTextNoTaskTitleCanHold(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{name: "oversized", text: strings.Repeat("é", models.MaxTaskTitleLength+1)},
		{name: "leading whitespace", text: " padded"},
		{name: "trailing whitespace", text: "padded "},
		{name: "newline", text: "first\nsecond"},
		{name: "markdown heading", text: "## Heading"},
		{name: "metadata comment open", text: "before <!-- after"},
		{name: "metadata comment close", text: "before --> after"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner := newTask("owner", 1, "Owner")
			owner.Checklist = []models.ChecklistItem{{Text: tc.text}}
			repo := newFakeRepo(newProject("p1", "Alpha", owner))
			svc := newTestService(repo)

			_, err := promoteFrozen(svc, repo.loc("owner"), owner, 0)
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if len(owner.Checklist) != 1 || owner.Checklist[0].Text != tc.text {
				t.Fatalf("refused promotion mutated the checklist: %+v", owner.Checklist)
			}
			requirePromotionChangedNothing(t, repo)
		})
	}
}

// TestPromoteChecklistItemRefusesBlankItemText covers the one invalid text the
// durable format cannot even hold, so the fixture is mutated after seeding: the
// guard still has to fire, because an item whose text is only whitespace is a
// title no task can carry and there is nothing to promote.
func TestPromoteChecklistItemRefusesBlankItemText(t *testing.T) {
	owner := newTask("owner", 1, "Owner")
	owner.Checklist = []models.ChecklistItem{{Text: "placeholder"}}
	repo := newFakeRepo(newProject("p1", "Alpha", owner))
	svc := newTestService(repo)
	owner.Checklist[0].Text = "   "

	_, err := promoteFrozen(svc, repo.loc("owner"), owner, 0)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	requirePromotionChangedNothing(t, repo)
}

// TestPromoteChecklistItemAcceptsUnicodeAtTheTitleLimit is the accepting half of
// the title rule: multi-byte text is measured in runes, not bytes, and survives
// the conversion exactly as written.
func TestPromoteChecklistItemAcceptsUnicodeAtTheTitleLimit(t *testing.T) {
	text := "日本語 ☕ " + strings.Repeat("é", models.MaxTaskTitleLength-7)
	owner := newTask("owner", 1, "Owner")
	owner.Checklist = []models.ChecklistItem{{Text: text}}
	repo := newFakeRepo(newProject("p1", "Alpha", owner))
	svc := newTestService(repo)

	res, err := promoteFrozen(svc, repo.loc("owner"), owner, 0)
	if err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}
	if got := repo.projects[0].GetTaskByID(res.Child.TaskID).Title; got != text {
		t.Fatalf("child title = %q, want the item's text byte-for-byte", got)
	}
}

// TestPromoteChecklistItemRefusesAtMaximumDepth proves the promotion reuses the
// one parent validation rather than a copy of it: an owner already at the deepest
// level cannot take a child, so the item is not consumed.
func TestPromoteChecklistItemRefusesAtMaximumDepth(t *testing.T) {
	chain := make([]*models.Task, 0, models.MaxNestingDepth+1)
	for depth := 0; depth <= models.MaxNestingDepth; depth++ {
		task := newTask(shortDepthID(depth), depth+1, "Level "+shortDepthID(depth))
		if depth > 0 {
			parentID := chain[depth-1].ID
			task.ParentID = &parentID
		}
		chain = append(chain, task)
	}
	owner := chain[models.MaxNestingDepth]
	owner.Checklist = []models.ChecklistItem{{Text: "Too deep to promote"}}
	repo := newFakeRepo(newProject("p1", "Alpha", chain...))
	svc := newTestService(repo)

	if owner.Level != models.MaxNestingDepth {
		t.Fatalf("fixture owner level = %d, want %d", owner.Level, models.MaxNestingDepth)
	}
	_, err := promoteFrozen(svc, repo.loc(owner.ID), owner, 0)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(owner.Checklist) != 1 {
		t.Fatalf("refused promotion consumed the item: %+v", owner.Checklist)
	}
	requirePromotionChangedNothing(t, repo)
}

// shortDepthID names the tasks of the depth fixture chain deterministically.
func shortDepthID(depth int) string { return string(rune('a' + depth)) }

// TestPromoteChecklistItemRefusesUnreachableOwner covers the two owners a
// promotion can name but not reach: one that does not exist, and one whose
// project is archived and therefore absent from the mutation read model.
func TestPromoteChecklistItemRefusesUnreachableOwner(t *testing.T) {
	archivedOwner := newTask("archived-owner", 1, "Archived owner")
	archivedOwner.Checklist = []models.ChecklistItem{{Text: "Unreachable"}}
	archived := newProject("p2", "Archived", archivedOwner)
	live := newTask("live", 1, "Live")
	repo := newFakeRepo(newProject("p1", "Alpha", live)).withUnloaded(archived)
	svc := newTestService(repo)

	cases := []struct {
		name    string
		locator app.TaskLocator
	}{
		{name: "missing task", locator: app.TaskLocator{ProjectID: "p1", TaskID: "nope"}},
		{name: "missing project", locator: app.TaskLocator{ProjectID: "nope", TaskID: "live"}},
		{name: "archived project", locator: app.TaskLocator{ProjectID: "p2", TaskID: "archived-owner"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.PromoteChecklistItem(app.ChecklistPromotionRequest{
				Owner: tc.locator, Index: 0, Checklist: []models.ChecklistItem{{Text: "Unreachable"}},
			})
			if !errors.Is(err, app.ErrTaskNotFound) {
				t.Fatalf("err = %v, want ErrTaskNotFound", err)
			}
			requirePromotionChangedNothing(t, repo)
		})
	}

	_, err := svc.PromoteChecklistItem(app.ChecklistPromotionRequest{Index: 0})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty locator err = %v, want ErrInvalidInput", err)
	}
	if len(archivedOwner.Checklist) != 1 {
		t.Fatalf("archived owner mutated: %+v", archivedOwner.Checklist)
	}
}

// TestPromoteChecklistItemRefusesConcurrentProjectChange exercises Session 1's
// strict read-set guard through the promotion: another writer advancing the
// owner's project between snapshot and commit refuses the whole transaction, and
// the rollback returns the checklist, the created child, and the consumed short
// ID to exactly the state the snapshot held.
func TestPromoteChecklistItemRefusesConcurrentProjectChange(t *testing.T) {
	repo, svc, owner := newPromotionFixture(t)
	project := repo.projects[0]
	nextShortID := project.NextShortID
	taskCount := len(project.Tasks)
	repo.preCommit = func() { repo.revisions[project.ID] = "moved-by-another-machine" }

	_, err := promoteFrozen(svc, repo.loc("owner"), owner, 1)
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("err = %v, want ErrStaleRevision", err)
	}
	var conflict *storage.RevisionConflictError
	if !errors.As(err, &conflict) || conflict.ProjectID != project.ID {
		t.Fatalf("conflict = %#v, want the owner's project named", conflict)
	}
	if !slicesEqualChecklist(owner.Checklist, promotionFixtureChecklist()) {
		t.Fatalf("refused promotion left the checklist as %+v", owner.Checklist)
	}
	if len(project.Tasks) != taskCount {
		t.Fatalf("project holds %d tasks, want the created child rolled back to %d",
			len(project.Tasks), taskCount)
	}
	if project.NextShortID != nextShortID {
		t.Fatalf("NextShortID = %d, want the consumed number rolled back to %d",
			project.NextShortID, nextShortID)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none recorded", len(repo.commits))
	}
}

// TestPromoteChecklistItemGuardsTheWholeReadSet proves the guard is a strict
// precondition rather than the ordinary merge expectation, so a refusal is
// decided before any three-way merge could absorb it.
func TestPromoteChecklistItemGuardsTheWholeReadSet(t *testing.T) {
	repo, svc, owner := newPromotionFixture(t)
	if _, err := promoteFrozen(svc, repo.loc("owner"), owner, 0); err != nil {
		t.Fatalf("PromoteChecklistItem: %v", err)
	}
	changeset, ok := repo.lastCommit()
	if !ok {
		t.Fatal("want exactly one changeset in the one commit")
	}
	if len(changeset.ProjectPreconditions) != 1 {
		t.Fatalf("preconditions = %+v, want one over the owner's project",
			changeset.ProjectPreconditions)
	}
	if changeset.ProjectPreconditions[0].ProjectID != repo.projects[0].ID {
		t.Fatalf("precondition names %q, want the owner's project",
			changeset.ProjectPreconditions[0].ProjectID)
	}
	if changeset.UndoAction == "" {
		t.Fatal("promotion must record a ledger entry")
	}
}

// TestPromoteChecklistItemRollsBackOnCommitFailure is the general refusal shape:
// whatever the commit boundary rejects, the workspace is the one the snapshot
// described — no child, no shortened checklist, no consumed short ID.
func TestPromoteChecklistItemRollsBackOnCommitFailure(t *testing.T) {
	repo, svc, owner := newPromotionFixture(t)
	repo.commitErr = errors.New("commit failed")
	project := repo.projects[0]
	nextShortID := project.NextShortID

	if _, err := promoteFrozen(svc, repo.loc("owner"), owner, 2); err == nil {
		t.Fatal("PromoteChecklistItem unexpectedly succeeded")
	}
	if !slicesEqualChecklist(owner.Checklist, promotionFixtureChecklist()) {
		t.Fatalf("checklist = %+v, want it restored", owner.Checklist)
	}
	if len(project.Tasks) != 1 || project.NextShortID != nextShortID {
		t.Fatalf("project = %d tasks / next %d, want the creation rolled back",
			len(project.Tasks), project.NextShortID)
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
