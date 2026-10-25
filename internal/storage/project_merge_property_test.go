package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func TestMerge_ProjectNameConflict(t *testing.T) {
	base := mergeTestDoc("base-name", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("local-name", cloneDocTasks(base.Tasks)...)
	current := mergeTestDoc("current-name", cloneDocTasks(base.Tasks)...)

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatal("divergent project name must conflict")
	}
	if !hasConflictKind(res, MergeConflictProjectMetadata) {
		t.Fatalf("expected project_metadata conflict, got %v", conflictKinds(res))
	}
}

func TestMerge_ProjectNameOneSideAdopts(t *testing.T) {
	base := mergeTestDoc("base-name", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("renamed", cloneDocTasks(base.Tasks)...)
	current := mergeTestDoc("base-name", cloneDocTasks(base.Tasks)...)

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("single-side rename should adopt, got: %+v", res.Conflicts)
	}
	if res.Merged.ProjectName != "renamed" {
		t.Errorf("project name = %q, want renamed", res.Merged.ProjectName)
	}
}

// --- invariant re-check after merge --------------------------------------

func TestMerge_InvariantTerminalWithoutCompletedAt(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	// local marks the task done (terminal) but leaves CompletedAt nil; current
	// leaves it untouched. The merged task is terminal with no timestamp.
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[0].WorkflowState = models.StateDone
	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatalf("terminal task without completion timestamp must be rejected, conflicts: %+v", res.Conflicts)
	}
	if !hasConflictKind(res, MergeConflictInvariant) {
		t.Fatalf("expected invariant conflict, got %v", conflictKinds(res))
	}
}

func TestMerge_TerminalWithCompletedAtPasses(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	completed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[0].WorkflowState = models.StateDone
	local.Tasks[0].CompletedAt = &completed
	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("consistent terminal task should merge, got: %+v", res.Conflicts)
	}
	if !mergedTaskByID(t, res, "a").IsTerminalState() {
		t.Error("merged task should be terminal")
	}
}

// --- property tests (deterministic, seeded) ------------------------------

// TestMerge_Property_IdentityBaseUnchanged verifies the algebraic identity: if
// local made no changes, the merge is exactly current; symmetrically for
// current. Both directions run over many seeded random field edits.
func TestMerge_Property_IdentityBaseUnchanged(t *testing.T) {
	rng := rand.New(rand.NewSource(20)) // fixed seed => deterministic
	for iter := 0; iter < 200; iter++ {
		base := mergeTestDoc("p",
			mergeTestTask("a", "Alpha"),
			mergeTestTask("b", "Beta"),
		)
		// current applies a random clean edit set (task-field edits + append).
		current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
		applyRandomTaskEdits(rng, current)
		// local is byte-for-byte base (no change).
		local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)

		res := mustMerge(t, base, local, current)
		if res.HasConflicts() {
			t.Fatalf("iter %d: local-unchanged merge must not conflict: %+v", iter, res.Conflicts)
		}
		if res.Merged.ComputeRevision() != current.ComputeRevision() {
			t.Fatalf("iter %d: local-unchanged merge must equal current", iter)
		}

		// Symmetric: current unchanged, local edited => merged == local.
		res2 := mustMerge(t, base, current /*local*/, mergeTestDoc("p", cloneDocTasks(base.Tasks)...) /*current==base*/)
		if res2.HasConflicts() {
			t.Fatalf("iter %d: current-unchanged merge must not conflict: %+v", iter, res2.Conflicts)
		}
		if res2.Merged.ComputeRevision() != current.ComputeRevision() {
			t.Fatalf("iter %d: current-unchanged merge must equal the edited side", iter)
		}
	}
}

// TestMerge_Property_Deterministic verifies running the same merge twice yields
// identical results (content and conflicts), and that swapping local/current
// yields the same merged content for conflict-free merges (order-independence
// of the field merge).
func TestMerge_Property_Deterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for iter := 0; iter < 200; iter++ {
		base := mergeTestDoc("p",
			mergeTestTask("a", "Alpha"),
			mergeTestTask("b", "Beta"),
			mergeTestTask("c", "Gamma"),
		)
		local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
		current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
		// Edit disjoint fields on disjoint tasks so the two sides stay mergeable.
		local.Tasks[0].Title = fmt.Sprintf("A-%d", rng.Intn(1000))
		current.Tasks[1].Priority = rng.Intn(3) + 1

		r1 := mustMerge(t, base, local, current)
		r2 := mustMerge(t, base, local, current)
		if (r1.Merged == nil) != (r2.Merged == nil) {
			t.Fatalf("iter %d: merge non-determinism in conflict outcome", iter)
		}
		if r1.Merged != nil && r1.Merged.ComputeRevision() != r2.Merged.ComputeRevision() {
			t.Fatalf("iter %d: repeated merge produced different content", iter)
		}

		// Swap local/current: content must be identical for a clean merge.
		swapped := mustMerge(t, base, current, local)
		if r1.Merged != nil {
			if swapped.Merged == nil {
				t.Fatalf("iter %d: swap flipped a clean merge into a conflict", iter)
			}
			if swapped.Merged.ComputeRevision() != r1.Merged.ComputeRevision() {
				t.Fatalf("iter %d: swapping local/current changed merged content", iter)
			}
		}
	}
}

// applyRandomTaskEdits applies a deterministic set of clean edits to a document:
// per-task field tweaks plus an optional appended task. It never reorders or
// deletes, so the result is always a clean superset the identity property can
// assert against.
func applyRandomTaskEdits(rng *rand.Rand, doc *ProjectDocument) {
	for _, task := range doc.Tasks {
		switch rng.Intn(3) {
		case 0:
			task.Title += fmt.Sprintf(" r%d", rng.Intn(1000))
		case 1:
			task.Priority = rng.Intn(3) + 1
		case 2:
			task.Description = fmt.Sprintf("desc-%d", rng.Intn(1000))
		}
	}
	if rng.Intn(2) == 0 {
		doc.Tasks = append(doc.Tasks, mergeTestTask(fmt.Sprintf("added-%d", rng.Intn(100000)), "Added"))
	}
}
