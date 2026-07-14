package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
)

// These tests pin the application use cases the sidebar-unlink, score-editor, and
// pinned-task overlays route through after session 33.7.3.4.5 retired their raw
// saves: ReplaceScoreDimensions (replace, not merge), RemoveRelatedBidirectional
// (both sides, atomic, spanning projects), and ReorderPinnedTasks (atomic swap).

func TestReplaceScoreDimensions_ReplacesNotMerges(t *testing.T) {
	task := newTask("t1", 1, "Scored")
	task.ScoreDimensions = map[string]int{"impact": 1, "reach": 2}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	// The score editor recomputes the full manual set on save; a dimension reset
	// to derived is simply absent, so replace must drop "reach", not keep it.
	res, err := svc.ReplaceScoreDimensions("t1", map[string]int{"impact": 4})
	if err != nil {
		t.Fatalf("ReplaceScoreDimensions: %v", err)
	}
	if res.TaskID != "t1" {
		t.Fatalf("result task = %q, want t1", res.TaskID)
	}
	if len(task.ScoreDimensions) != 1 || task.ScoreDimensions["impact"] != 4 {
		t.Fatalf("ScoreDimensions = %v, want exactly {impact:4}", task.ScoreDimensions)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Set Score" {
		t.Fatalf("lastCommit = %+v ok=%v, want one Set Score commit", cs, ok)
	}

	// An empty set clears every manual dimension (fully derived task) and still
	// commits — this is the "reset all" save the merge-only SetScoreDimensions
	// rejects.
	if _, err := svc.ReplaceScoreDimensions("t1", nil); err != nil {
		t.Fatalf("ReplaceScoreDimensions(empty): %v", err)
	}
	if len(task.ScoreDimensions) != 0 {
		t.Fatalf("ScoreDimensions after clear = %v, want empty", task.ScoreDimensions)
	}
}

func TestRemoveRelatedBidirectional_SameProject(t *testing.T) {
	t1 := newTask("t1", 1, "Source")
	t2 := newTask("t2", 2, "Peer")
	t1.RelatedTo = []string{"t2"}
	t2.RelatedTo = []string{"t1"}
	repo := newFakeRepo(newProject("p1", "Alpha", t1, t2))
	svc := newTestService(repo)

	res, err := svc.RemoveRelatedBidirectional("t1", "t2")
	if err != nil {
		t.Fatalf("RemoveRelatedBidirectional: %v", err)
	}
	if len(t1.RelatedTo) != 0 || len(t2.RelatedTo) != 0 {
		t.Fatalf("related not cleared on both sides: t1=%v t2=%v", t1.RelatedTo, t2.RelatedTo)
	}
	if len(res.TouchedProjectIDs) != 1 || res.TouchedProjectIDs[0] != "p1" {
		t.Fatalf("touched = %v, want [p1]", res.TouchedProjectIDs)
	}
	last := repo.commits[len(repo.commits)-1]
	if len(last) != 1 {
		t.Fatalf("same-project removal committed %d changesets, want 1", len(last))
	}
}

func TestRemoveRelatedBidirectional_CrossProjectAtomic(t *testing.T) {
	t1 := newTask("t1", 1, "Source")
	t2 := newTask("t2", 2, "Peer")
	t1.RelatedTo = []string{"p2:t2"}
	t2.RelatedTo = []string{"p1:t1"}
	repo := newFakeRepo(newProject("p1", "Alpha", t1), newProject("p2", "Beta", t2))
	svc := newTestService(repo)

	res, err := svc.RemoveRelatedBidirectional("t1", "p2:t2")
	if err != nil {
		t.Fatalf("RemoveRelatedBidirectional: %v", err)
	}
	if len(t1.RelatedTo) != 0 || len(t2.RelatedTo) != 0 {
		t.Fatalf("cross-project related not cleared: t1=%v t2=%v", t1.RelatedTo, t2.RelatedTo)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Fatalf("touched = %v, want both projects", res.TouchedProjectIDs)
	}
	// Both project writes must land in ONE Commit call (one atomic transaction),
	// so a cross-project unlink can never leave one side dangling.
	last := repo.commits[len(repo.commits)-1]
	if len(last) != 2 {
		t.Fatalf("cross-project removal committed %d changesets, want 2 in one transaction", len(last))
	}
}

func TestRelatedBidirectionalMatchesLegacySlugAndStableIDIdentity(t *testing.T) {
	t1 := newTask("t1", 1, "Source")
	t2 := newTask("t2", 2, "Peer")
	// Both persisted spellings pre-exist on both sides. Adding through the
	// stable-ID TUI producer must collapse them by identity, and removal through
	// that same spelling must clear the retained legacy slug aliases too.
	t1.RelatedTo = []string{"beta:t2", "p2:t2"}
	t2.RelatedTo = []string{"alpha:t1", "p1:t1"}
	repo := newFakeRepo(newProject("p1", "Alpha", t1), newProject("p2", "Beta", t2))
	svc := newTestService(repo)

	if _, err := svc.AddRelatedBidirectional("t1", "p2:t2"); err != nil {
		t.Fatalf("AddRelatedBidirectional: %v", err)
	}
	if len(t1.RelatedTo) != 1 || t1.RelatedTo[0] != "beta:t2" {
		t.Fatalf("source identity dedup = %v, want [beta:t2]", t1.RelatedTo)
	}
	if len(t2.RelatedTo) != 1 || t2.RelatedTo[0] != "alpha:t1" {
		t.Fatalf("target identity dedup = %v, want [alpha:t1]", t2.RelatedTo)
	}

	if _, err := svc.RemoveRelatedBidirectional("t1", "p2:t2"); err != nil {
		t.Fatalf("RemoveRelatedBidirectional: %v", err)
	}
	if len(t1.RelatedTo) != 0 || len(t2.RelatedTo) != 0 {
		t.Fatalf("identity removal left refs: source=%v target=%v", t1.RelatedTo, t2.RelatedTo)
	}
}

func TestRemoveRelatedBidirectional_MissingPeerDegradesToSourceOnly(t *testing.T) {
	t1 := newTask("t1", 1, "Source")
	t1.RelatedTo = []string{"p2:gone"}
	repo := newFakeRepo(newProject("p1", "Alpha", t1))
	svc := newTestService(repo)

	res, err := svc.RemoveRelatedBidirectional("t1", "p2:gone")
	if err != nil {
		t.Fatalf("RemoveRelatedBidirectional: %v", err)
	}
	if len(t1.RelatedTo) != 0 {
		t.Fatalf("source ref not removed: %v", t1.RelatedTo)
	}
	if len(res.TouchedProjectIDs) != 1 || res.TouchedProjectIDs[0] != "p1" {
		t.Fatalf("touched = %v, want [p1] (source only)", res.TouchedProjectIDs)
	}
}

func TestReorderPinnedTasks_CrossProjectAtomicSwap(t *testing.T) {
	early := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	t1 := newTask("t1", 1, "First")
	t2 := newTask("t2", 2, "Second")
	t1.Pinned, t1.PinnedAt = true, &early
	t2.Pinned, t2.PinnedAt = true, &late
	repo := newFakeRepo(newProject("p1", "Alpha", t1), newProject("p2", "Beta", t2))
	svc := newTestService(repo)

	res, err := svc.ReorderPinnedTasks("t1", "t2")
	if err != nil {
		t.Fatalf("ReorderPinnedTasks: %v", err)
	}
	if t1.PinnedAt == nil || !t1.PinnedAt.Equal(late) {
		t.Fatalf("t1 PinnedAt = %v, want %v (took t2's slot)", t1.PinnedAt, late)
	}
	if t2.PinnedAt == nil || !t2.PinnedAt.Equal(early) {
		t.Fatalf("t2 PinnedAt = %v, want %v (took t1's slot)", t2.PinnedAt, early)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Fatalf("touched = %v, want both projects", res.TouchedProjectIDs)
	}
	last := repo.commits[len(repo.commits)-1]
	if len(last) != 2 {
		t.Fatalf("reorder committed %d changesets, want 2 in one transaction", len(last))
	}
}

func TestReorderPinnedTasks_EqualTimesTieBreak(t *testing.T) {
	same := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	t1 := newTask("t1", 1, "First")
	t2 := newTask("t2", 2, "Second")
	t1.Pinned, t1.PinnedAt = true, &same
	t2.Pinned, t2.PinnedAt = true, &same
	repo := newFakeRepo(newProject("p1", "Alpha", t1, t2))
	svc := newTestService(repo)

	if _, err := svc.ReorderPinnedTasks("t1", "t2"); err != nil {
		t.Fatalf("ReorderPinnedTasks: %v", err)
	}
	// Equal timestamps must produce a strict ordering rather than a no-op.
	if t1.PinnedAt.Equal(*t2.PinnedAt) {
		t.Fatalf("equal PinnedAt left unbroken: t1=%v t2=%v", t1.PinnedAt, t2.PinnedAt)
	}
}

func TestReorderPinnedTasks_RequiresBothPinned(t *testing.T) {
	pinned := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	t1 := newTask("t1", 1, "First")
	t2 := newTask("t2", 2, "Second")
	t1.Pinned, t1.PinnedAt = true, &pinned
	// t2 deliberately not pinned.
	repo := newFakeRepo(newProject("p1", "Alpha", t1, t2))
	svc := newTestService(repo)

	if _, err := svc.ReorderPinnedTasks("t1", "t2"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput when a target is not pinned", err)
	}
	if len(repo.commits) != 0 {
		t.Fatal("a rejected reorder must not commit")
	}
}
