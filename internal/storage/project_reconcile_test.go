package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func reconcileTestBase() *ProjectDocument {
	parent := "a"
	child := mergeTestTask("c", "Gamma")
	child.ParentID = &parent
	return mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"), child, mergeTestTask("d", "Delta"))
}

func mustReconcile(t *testing.T, base *ProjectDocument, sides ...*ProjectDocument) (*ProjectDocument, []MergeConflict) {
	t.Helper()
	merged, conflicts, err := ReconcileProjectDocuments(base, sides)
	if err != nil {
		t.Fatalf("ReconcileProjectDocuments: %v", err)
	}
	return merged, conflicts
}

// TestReconcileProjectDocumentsAgreesWithTwoWayMerge holds the sync merge to
// the existing two-way merge wherever that merge is clean: same content, no
// conflicts.
func TestReconcileProjectDocumentsAgreesWithTwoWayMerge(t *testing.T) {
	rng := rand.New(rand.NewSource(55))
	clean := 0
	for iter := 0; iter < 300; iter++ {
		base := reconcileTestBase()
		local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
		current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
		randomReconcileEdit(rng, local, "l", iter)
		randomReconcileEdit(rng, current, "r", iter)
		two := mustMerge(t, base, local, current)
		if two.HasConflicts() {
			continue
		}
		clean++
		merged, conflicts := mustReconcile(t, base, local, current)
		if len(conflicts) > 0 {
			t.Fatalf("iter %d: a clean two-way merge reconciled with conflicts %+v", iter, conflicts)
		}
		if merged.ComputeRevision() != two.Merged.ComputeRevision() {
			want, _ := two.Merged.WriteCurrent()
			got, _ := merged.WriteCurrent()
			t.Fatalf("iter %d: reconcile differs from the two-way merge:\n%s\n---\n%s", iter, want, got)
		}
	}
	if clean < 50 {
		t.Fatalf("only %d of 300 seeded merges were clean; the property is not exercised", clean)
	}
}

// TestReconcileProjectDocumentsAlwaysYieldsValidDocuments throws seeded
// competing edits, deletions, reparenting, lifecycle changes and short-id
// collisions at the merge from up to three sides and requires a valid,
// renderable, deterministic document every time, with a conflict reported
// whenever the two-way merge would have refused for a reason other than the
// short ids reconciliation renumbers.
func TestReconcileProjectDocumentsAlwaysYieldsValidDocuments(t *testing.T) {
	rng := rand.New(rand.NewSource(77))
	conflicted := 0
	for iter := 0; iter < 400; iter++ {
		base := reconcileTestBase()
		sides := make([]*ProjectDocument, 2+rng.Intn(2))
		for i := range sides {
			sides[i] = mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
			randomReconcileEdit(rng, sides[i], fmt.Sprintf("s%d", i), iter)
			randomReconcileEdit(rng, sides[i], fmt.Sprintf("t%d", i), iter)
		}
		merged, conflicts := mustReconcile(t, base, sides...)
		if audit := auditMergedDocument(merged); len(audit) > 0 {
			t.Fatalf("iter %d: reconciled document is invalid: %+v", iter, audit)
		}
		if _, err := merged.WriteCurrent(); err != nil {
			t.Fatalf("iter %d: reconciled document does not render: %v", iter, err)
		}
		again, againConflicts := mustReconcile(t, base, sides...)
		if again.ComputeRevision() != merged.ComputeRevision() || fmt.Sprint(againConflicts) != fmt.Sprint(conflicts) {
			t.Fatalf("iter %d: reconciliation is not deterministic", iter)
		}
		two := mustMerge(t, base, sides[0], sides[1])
		refused := false
		for _, c := range two.Conflicts {
			refused = refused || c.Field != "short_id"
		}
		if len(sides) == 2 && refused && len(conflicts) == 0 {
			t.Fatalf("iter %d: the two-way merge refused %+v but reconciliation reported nothing", iter, two.Conflicts)
		}
		if len(conflicts) > 0 {
			conflicted++
		}
	}
	if conflicted < 50 {
		t.Fatalf("only %d of 400 seeded merges conflicted; the property is not exercised", conflicted)
	}
}

// randomReconcileEdit applies one seeded edit that leaves doc itself valid.
func randomReconcileEdit(rng *rand.Rand, doc *ProjectDocument, tag string, iter int) {
	task := doc.Tasks[rng.Intn(len(doc.Tasks))]
	switch rng.Intn(8) {
	case 0:
		task.Title = fmt.Sprintf("%s title %d", tag, rng.Intn(2))
	case 1:
		task.Priority = rng.Intn(3) + 1
	case 2:
		added := mergeTestTask(fmt.Sprintf("new-%s-%d", tag, iter), "Added "+tag)
		added.ShortID = 50
		doc.Tasks = append(doc.Tasks, added)
	case 3:
		for _, other := range doc.Tasks {
			if other.ParentID != nil && *other.ParentID == task.ID {
				return
			}
		}
		kept := doc.Tasks[:0]
		for _, other := range doc.Tasks {
			if other != task {
				kept = append(kept, other)
			}
		}
		doc.Tasks = kept
	case 4:
		target := doc.Tasks[rng.Intn(len(doc.Tasks))]
		taskMap := map[string]*models.Task{}
		for _, other := range doc.Tasks {
			taskMap[other.ID] = other
		}
		if target != task && (!target.IsTerminalState() || task.IsTerminalState()) && !models.WouldCreateCycle(taskMap, task.ID, target.ID) {
			id := target.ID
			task.ParentID = &id
		}
	case 5:
		for _, other := range doc.Tasks {
			if other.ParentID != nil && *other.ParentID == task.ID && !other.IsTerminalState() {
				return
			}
		}
		completed := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
		task.WorkflowState = models.StateDone
		task.CompletedAt = &completed
		task.NextCheckIn = nil
		task.CheckInRepeatDays = 0
	case 6:
		if !task.IsTerminalState() {
			next := time.Date(2026, 7, 10, 0, 0, 0, 0, time.Local)
			task.NextCheckIn = &next
		}
	case 7:
		if len(doc.Tasks) > 1 {
			doc.Tasks[0], doc.Tasks[len(doc.Tasks)-1] = doc.Tasks[len(doc.Tasks)-1], doc.Tasks[0]
		}
	}
}
