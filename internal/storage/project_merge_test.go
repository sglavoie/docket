package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// --- test fixtures -------------------------------------------------------

func mergeTestTask(id, title string) *models.Task {
	stamp := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	return &models.Task{
		ID:             id,
		ShortID:        1,
		Title:          title,
		WorkflowState:  models.StateBacklog,
		Priority:       models.PriorityUnset,
		Tags:           []string{},
		CreatedAt:      stamp,
		UpdatedAt:      stamp,
		StateChangedAt: stamp,
	}
}

func mergeTestDoc(name string, tasks ...*models.Task) *ProjectDocument {
	doc := &ProjectDocument{
		ProjectName:   name,
		Tasks:         tasks,
		SchemaVersion: SchemaVersionV11,
	}
	doc.Revision = doc.ComputeRevision()
	return doc
}

func TestMergeableTaskFieldsIncludesContext(t *testing.T) {
	for _, field := range mergeableTaskFields {
		if field.jsonName == "context" {
			return
		}
	}
	t.Fatal("merge field inventory does not include context")
}

func mergeTaskIDs(tasks []*models.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

// cloneDocTasks deep-copies the task slice (and each task value) so a test can
// mutate one branch's tasks without disturbing the base fixture.
func cloneDocTasks(tasks []*models.Task) []*models.Task {
	out := make([]*models.Task, len(tasks))
	for i, t := range tasks {
		copied := *t
		out[i] = &copied
	}
	return out
}

func mustMerge(t *testing.T, base, local, current *ProjectDocument) *ProjectMergeResult {
	t.Helper()
	res, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments returned error: %v", err)
	}
	return res
}

func conflictKinds(res *ProjectMergeResult) []MergeConflictKind {
	kinds := make([]MergeConflictKind, len(res.Conflicts))
	for i, c := range res.Conflicts {
		kinds[i] = c.Kind
	}
	return kinds
}

func hasConflictKind(res *ProjectMergeResult, kind MergeConflictKind) bool {
	for _, c := range res.Conflicts {
		if c.Kind == kind {
			return true
		}
	}
	return false
}

func mergedTaskByID(t *testing.T, res *ProjectMergeResult, id string) *models.Task {
	t.Helper()
	if res.Merged == nil {
		t.Fatalf("expected a merged document, got conflicts: %+v", res.Conflicts)
	}
	for _, task := range res.Merged.Tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// --- input validation ----------------------------------------------------

func TestMergeProjectDocuments_NilInputs(t *testing.T) {
	valid := mergeTestDoc("p")
	cases := []struct {
		name                 string
		base, local, current *ProjectDocument
	}{
		{"nil base", nil, valid, valid},
		{"nil local", valid, nil, valid},
		{"nil current", valid, valid, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MergeProjectDocuments(tc.base, tc.local, tc.current); err == nil {
				t.Fatal("expected error for nil input, got nil")
			}
		})
	}
}

// --- hand-authored merge matrix ------------------------------------------

func TestMerge_NoChangesReturnsBase(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("unchanged merge should not conflict: %+v", res.Conflicts)
	}
	if res.Merged.ComputeRevision() != base.ComputeRevision() {
		t.Fatalf("unchanged merge should equal base revision")
	}
}

func TestMerge_IndependentFieldEditsAutoMerge(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))

	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[0].Title = "Alpha edited" // local changes title

	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current.Tasks[0].Priority = 1 // current changes a different field

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("independent field edits should auto-merge, got: %+v", res.Conflicts)
	}
	merged := mergedTaskByID(t, res, "a")
	if merged.Title != "Alpha edited" {
		t.Errorf("title = %q, want local edit", merged.Title)
	}
	if merged.Priority != 1 {
		t.Errorf("priority = %d, want current edit", merged.Priority)
	}
}

func TestMerge_IndependentEditsWithDerivedMetadataAutoMerge(t *testing.T) {
	baseTime := time.Date(2026, 7, 14, 9, 0, 0, 0, time.Local)
	currentTime := baseTime.Add(time.Hour)
	localTime := baseTime.Add(2 * time.Hour)

	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	base.Tasks[0].CreatedAt = baseTime
	base.Tasks[0].UpdatedAt = baseTime

	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[0].Title = "Alpha edited"
	local.Tasks[0].UpdatedAt = localTime

	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current.Tasks[0].Priority = models.PriorityMin
	current.Tasks[0].UpdatedAt = currentTime

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("derived metadata must not conflict with independent edits: %+v", res.Conflicts)
	}
	merged := mergedTaskByID(t, res, "a")
	if merged.Title != "Alpha edited" || merged.Priority != models.PriorityMin {
		t.Fatalf("merged business fields = title %q priority %d", merged.Title, merged.Priority)
	}
	if !merged.UpdatedAt.Equal(localTime) {
		t.Fatalf("merged UpdatedAt = %v, want latest %v", merged.UpdatedAt, localTime)
	}
}

func TestMerge_IdenticalConcurrentEditsCoalesce(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[0].Title = "Same"
	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current.Tasks[0].Title = "Same"

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("identical concurrent edits should coalesce, got: %+v", res.Conflicts)
	}
	if got := mergedTaskByID(t, res, "a").Title; got != "Same" {
		t.Errorf("title = %q, want coalesced %q", got, "Same")
	}
}

func TestMerge_DivergentFieldEditConflicts(t *testing.T) {
	baseTime := time.Date(2026, 7, 14, 9, 0, 0, 0, time.Local)
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	base.Tasks[0].UpdatedAt = baseTime
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[0].Title = "Local"
	local.Tasks[0].UpdatedAt = baseTime.Add(time.Hour)
	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current.Tasks[0].Title = "Current"
	current.Tasks[0].UpdatedAt = baseTime.Add(2 * time.Hour)

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatal("conflicting merge must not produce a document")
	}
	if !hasConflictKind(res, MergeConflictFieldEdit) {
		t.Fatalf("expected field_edit conflict, got %v", conflictKinds(res))
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("expected only the user-authored title conflict, got %+v", res.Conflicts)
	}
	c := res.Conflicts[0]
	if c.Field != "title" || c.Local != "Local" || c.Current != "Current" || c.Base != "Alpha" {
		t.Errorf("conflict detail unexpected: %+v", c)
	}
}

func TestMerge_DeleteVersusUnchangedDeletes(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))
	// local leaves both, current deletes b (unchanged on local).
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current := mergeTestDoc("p", mergeTestTask("a", "Alpha"))

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("delete-vs-unchanged should resolve as deletion, got: %+v", res.Conflicts)
	}
	if mergedTaskByID(t, res, "b") != nil {
		t.Error("task b should have been deleted")
	}
	if mergedTaskByID(t, res, "a") == nil {
		t.Error("task a should survive")
	}
}

func TestMerge_DeleteVersusEditConflicts(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))
	// local edits b, current deletes b.
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[1].Title = "Beta edited"
	current := mergeTestDoc("p", mergeTestTask("a", "Alpha"))

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatal("delete-vs-edit must not produce a document")
	}
	if !hasConflictKind(res, MergeConflictDeleteEdit) {
		t.Fatalf("expected delete_edit conflict, got %v", conflictKinds(res))
	}
}

func TestMerge_BothDeletedDrops(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))
	local := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	current := mergeTestDoc("p", mergeTestTask("a", "Alpha"))

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("both-deleted should be clean, got: %+v", res.Conflicts)
	}
	if len(res.Merged.Tasks) != 1 {
		t.Fatalf("merged tasks = %d, want 1", len(res.Merged.Tasks))
	}
}

func TestMerge_IndependentAdditionsMerge(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))
	current := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("c", "Gamma"))

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("independent additions should merge, got: %+v", res.Conflicts)
	}
	if len(res.Merged.Tasks) != 3 {
		t.Fatalf("merged tasks = %d, want 3 (a,b,c)", len(res.Merged.Tasks))
	}
	if mergedTaskByID(t, res, "b") == nil || mergedTaskByID(t, res, "c") == nil {
		t.Error("both additions should survive")
	}
}

func TestMerge_AddAddIdenticalCoalesces(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))
	current := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta"))

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("identical add/add should coalesce, got: %+v", res.Conflicts)
	}
	if len(res.Merged.Tasks) != 2 {
		t.Fatalf("merged tasks = %d, want 2", len(res.Merged.Tasks))
	}
}

func TestMerge_AddAddDivergentConflicts(t *testing.T) {
	base := mergeTestDoc("p", mergeTestTask("a", "Alpha"))
	local := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta local"))
	current := mergeTestDoc("p", mergeTestTask("a", "Alpha"), mergeTestTask("b", "Beta current"))

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatal("divergent add/add must not produce a document")
	}
	if !hasConflictKind(res, MergeConflictFieldEdit) {
		t.Fatalf("expected field_edit conflict on divergent add, got %v", conflictKinds(res))
	}
}

func TestMerge_ParentEditIsHierarchyConflict(t *testing.T) {
	parentA := "a"
	parentB := "b"
	base := mergeTestDoc("p",
		mergeTestTask("a", "Alpha"),
		mergeTestTask("b", "Beta"),
		func() *models.Task { s := mergeTestTask("c", "Gamma"); return s }(),
	)
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	local.Tasks[2].ParentID = &parentA
	current := mergeTestDoc("p", cloneDocTasks(base.Tasks)...)
	current.Tasks[2].ParentID = &parentB

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatal("divergent parent edits must not produce a document")
	}
	if !hasConflictKind(res, MergeConflictHierarchy) {
		t.Fatalf("expected hierarchy conflict, got %v", conflictKinds(res))
	}
}

func TestMerge_OneSideReordersAdopts(t *testing.T) {
	base := mergeTestDoc("p",
		mergeTestTask("a", "Alpha"),
		mergeTestTask("b", "Beta"),
		mergeTestTask("c", "Gamma"),
	)
	local := mergeTestDoc("p", cloneDocTasks(base.Tasks)...) // unchanged order
	// current reorders: c, a, b
	current := mergeTestDoc("p",
		mergeTestTask("c", "Gamma"),
		mergeTestTask("a", "Alpha"),
		mergeTestTask("b", "Beta"),
	)

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("single-side reorder should adopt, got: %+v", res.Conflicts)
	}
	gotOrder := []string{res.Merged.Tasks[0].ID, res.Merged.Tasks[1].ID, res.Merged.Tasks[2].ID}
	want := []string{"c", "a", "b"}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("order = %v, want %v", gotOrder, want)
		}
	}
}

func TestMerge_BothReorderIncompatiblyConflicts(t *testing.T) {
	base := mergeTestDoc("p",
		mergeTestTask("a", "Alpha"),
		mergeTestTask("b", "Beta"),
		mergeTestTask("c", "Gamma"),
	)
	local := mergeTestDoc("p",
		mergeTestTask("b", "Beta"),
		mergeTestTask("a", "Alpha"),
		mergeTestTask("c", "Gamma"),
	)
	current := mergeTestDoc("p",
		mergeTestTask("c", "Gamma"),
		mergeTestTask("b", "Beta"),
		mergeTestTask("a", "Alpha"),
	)

	res := mustMerge(t, base, local, current)
	if res.Merged != nil {
		t.Fatal("incompatible reorders must not produce a document")
	}
	if !hasConflictKind(res, MergeConflictHierarchy) {
		t.Fatalf("expected hierarchy (order) conflict, got %v", conflictKinds(res))
	}
}

func TestMerge_BothReorderIdenticallyCoalesces(t *testing.T) {
	base := mergeTestDoc("p",
		mergeTestTask("a", "Alpha"),
		mergeTestTask("b", "Beta"),
		mergeTestTask("c", "Gamma"),
	)
	reordered := []*models.Task{
		mergeTestTask("c", "Gamma"),
		mergeTestTask("a", "Alpha"),
		mergeTestTask("b", "Beta"),
	}
	local := mergeTestDoc("p", cloneDocTasks(reordered)...)
	current := mergeTestDoc("p", cloneDocTasks(reordered)...)

	res := mustMerge(t, base, local, current)
	if res.HasConflicts() {
		t.Fatalf("identical reorders should coalesce, got: %+v", res.Conflicts)
	}
	if res.Merged.Tasks[0].ID != "c" {
		t.Errorf("order not adopted, first = %q", res.Merged.Tasks[0].ID)
	}
}

func TestMerge_IndependentSiblingGroupReordersCoalesce(t *testing.T) {
	parentA := mergeTestTask("parent-a", "Parent A")
	parentB := mergeTestTask("parent-b", "Parent B")
	a1 := mergeTestTask("a1", "A1")
	a2 := mergeTestTask("a2", "A2")
	b1 := mergeTestTask("b1", "B1")
	b2 := mergeTestTask("b2", "B2")
	a1.ParentID = &parentA.ID
	a2.ParentID = &parentA.ID
	b1.ParentID = &parentB.ID
	b2.ParentID = &parentB.ID
	base := mergeTestDoc("p", parentA, a1, a2, parentB, b1, b2)

	localTasks := cloneDocTasks(base.Tasks)
	local := mergeTestDoc("p", localTasks[0], localTasks[2], localTasks[1], localTasks[3], localTasks[4], localTasks[5])
	currentTasks := cloneDocTasks(base.Tasks)
	current := mergeTestDoc("p", currentTasks[0], currentTasks[1], currentTasks[2], currentTasks[3], currentTasks[5], currentTasks[4])

	result := mustMerge(t, base, local, current)
	if result.HasConflicts() {
		t.Fatalf("independent sibling reorders conflicted: %+v", result.Conflicts)
	}
	mergedProject := &models.Project{Tasks: result.Merged.Tasks}
	mergedProject.BuildTaskTree()
	assertMergedOrder := func(task *models.Task, want ...string) {
		t.Helper()
		siblings := mergedProject.GetSiblings(task)
		if len(siblings) != len(want) {
			t.Fatalf("siblings of %s = %d, want %d", task.ID, len(siblings), len(want))
		}
		for i, id := range want {
			if siblings[i].ID != id {
				t.Fatalf("siblings of %s = %v, want %v", task.ID, mergeTaskIDs(siblings), want)
			}
		}
	}
	assertMergedOrder(mergedProject.GetTaskByID("a1"), "a2", "a1")
	assertMergedOrder(mergedProject.GetTaskByID("b1"), "b2", "b1")
}

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
