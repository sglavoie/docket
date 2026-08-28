package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestMergeableTaskFieldsIncludesDependencies proves the reflective inventory
// admitted the new persisted field on its own. Nothing enumerates merge fields
// by hand, so this is what keeps a dependency edit from being silently dropped
// by a concurrent commit.
func TestMergeableTaskFieldsIncludesDependencies(t *testing.T) {
	for _, field := range mergeableTaskFields {
		if field.jsonName == "dependencies" {
			return
		}
	}
	t.Fatal("merge field inventory does not include dependencies")
}

// mergeDependencyTask builds the one task every merge case here edits, so its
// identity is fixed: the contract under test is the dependency slice alone.
func mergeDependencyTask(dependencies ...models.TaskReference) *models.Task {
	task := mergeTestTask("t1", "Task")
	task.Dependencies = models.CloneTaskReferences(dependencies)
	return task
}

// TestMergeAdoptsAOneSidedDependencyEdit is the ordinary case: one side changed
// the list, the other did not, so the change lands without a conflict.
func TestMergeAdoptsAOneSidedDependencyEdit(t *testing.T) {
	added := models.TaskReference{ProjectID: "p", TaskID: "target"}
	base := mergeTestDoc("Deps", mergeDependencyTask())
	local := mergeTestDoc("Deps", mergeDependencyTask(added))
	current := mergeTestDoc("Deps", mergeDependencyTask())

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("one-sided dependency edit conflicted: %+v", result.Conflicts)
	}
	if !models.TaskReferencesEqual(result.Merged.Tasks[0].Dependencies, []models.TaskReference{added}) {
		t.Fatalf("merged dependencies = %+v, want %+v", result.Merged.Tasks[0].Dependencies, []models.TaskReference{added})
	}
}

// TestMergeCoalescesIdenticalDependencyEdits keeps a duplicated edit from being
// reported as divergence.
func TestMergeCoalescesIdenticalDependencyEdits(t *testing.T) {
	added := models.TaskReference{ProjectID: "p", TaskID: "target"}
	base := mergeTestDoc("Deps", mergeDependencyTask())
	local := mergeTestDoc("Deps", mergeDependencyTask(added))
	current := mergeTestDoc("Deps", mergeDependencyTask(added))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("identical dependency edits conflicted: %+v", result.Conflicts)
	}
	if !models.TaskReferencesEqual(result.Merged.Tasks[0].Dependencies, []models.TaskReference{added}) {
		t.Fatalf("merged dependencies = %+v", result.Merged.Tasks[0].Dependencies)
	}
}

// TestMergeReportsDivergentDependencyListsAsAWholeFieldConflict pins the locked
// contract: the dependency list is one field, so two divergent edits conflict as
// a whole rather than being element-wise unioned.
func TestMergeReportsDivergentDependencyListsAsAWholeFieldConflict(t *testing.T) {
	base := mergeTestDoc("Deps", mergeDependencyTask())
	local := mergeTestDoc("Deps", mergeDependencyTask(models.TaskReference{ProjectID: "p", TaskID: "left"}))
	current := mergeTestDoc("Deps", mergeDependencyTask(models.TaskReference{ProjectID: "p", TaskID: "right"}))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.Merged != nil {
		t.Fatal("a conflicting merge produced a document")
	}
	found := false
	for _, conflict := range result.Conflicts {
		if conflict.Field == "dependencies" {
			if conflict.Kind != MergeConflictFieldEdit {
				t.Fatalf("dependency conflict kind = %q, want %q", conflict.Kind, MergeConflictFieldEdit)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("divergent dependency lists produced no dependencies conflict: %+v", result.Conflicts)
	}
}

// TestMergedDependenciesDetachFromTheirSource covers the aliasing hazard the
// shallow task clone would otherwise leave: the merged document flows into a
// live project, so its locator slice must not share backing storage with the
// input documents.
func TestMergedDependenciesDetachFromTheirSource(t *testing.T) {
	added := models.TaskReference{ProjectID: "p", TaskID: "target"}
	base := mergeTestDoc("Deps", mergeDependencyTask())
	local := mergeTestDoc("Deps", mergeDependencyTask(added))
	current := mergeTestDoc("Deps", mergeDependencyTask())

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	merged := result.Merged.Tasks[0]
	merged.Dependencies[0].TaskID = "mutated"
	if local.Tasks[0].Dependencies[0].TaskID != "target" {
		t.Fatal("mutating the merged dependency list reached the local document")
	}
}

// TestConflictStorageRoundTripsDependencies proves a dependency edit survives
// the conflict bundle, which renders each of the three documents through the
// live writer and reads them back.
func TestConflictStorageRoundTripsDependencies(t *testing.T) {
	store := newConflictStore(t)
	added := models.TaskReference{ProjectID: "p", TaskID: "target"}

	base := mergeTestDoc("Deps", mergeDependencyTask())
	local := mergeTestDoc("Deps", mergeDependencyTask(added))
	remote := mergeTestDoc("Deps", mergeDependencyTask(models.TaskReference{ProjectID: "p", TaskID: "other"}))
	conflicts := []MergeConflict{{Kind: MergeConflictFieldEdit, Field: "dependencies"}}

	project := &models.Project{ID: "p1", Name: "Deps", Slug: "deps", Path: "/tmp/projects/deps"}
	id, err := RecordMergeConflict(store, project, base, local, remote, conflicts, "dependency edit", "adding a dependency")
	if err != nil {
		t.Fatalf("RecordMergeConflict: %v", err)
	}
	bundle, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !models.TaskReferencesEqual(bundle.Local.Tasks[0].Dependencies, []models.TaskReference{added}) {
		t.Fatalf("stored local dependencies = %+v, want %+v", bundle.Local.Tasks[0].Dependencies, []models.TaskReference{added})
	}
	if len(bundle.Base.Tasks[0].Dependencies) != 0 {
		t.Fatalf("stored base dependencies = %+v, want none", bundle.Base.Tasks[0].Dependencies)
	}
}
