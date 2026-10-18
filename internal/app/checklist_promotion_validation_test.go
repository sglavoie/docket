package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
