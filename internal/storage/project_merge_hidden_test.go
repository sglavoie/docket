package storage

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// TestMergeableTaskFieldsIncludesHidden proves the reflective inventory admitted
// the hidden flag as an ordinary field-wise merge field, outside the check-in
// unit.
func TestMergeableTaskFieldsIncludesHidden(t *testing.T) {
	for _, field := range mergeableTaskFields {
		if field.jsonName == "hidden" {
			if field.checkInUnit {
				t.Fatal("hidden is merged with the check-in unit")
			}
			return
		}
	}
	t.Fatal("merge field inventory does not include hidden")
}

func mergeHiddenTask(hidden bool, title string) *models.Task {
	task := mergeTestTask("t1", title)
	task.Hidden = hidden
	return task
}

// TestMergeAdoptsAOneSidedHideBesideAnotherFieldEdit is the ordinary case: one
// side hid the task while the other retitled it, and both edits land without a
// conflict.
func TestMergeAdoptsAOneSidedHideBesideAnotherFieldEdit(t *testing.T) {
	base := mergeTestDoc("Hidden", mergeHiddenTask(false, "Task"))
	local := mergeTestDoc("Hidden", mergeHiddenTask(true, "Task"))
	current := mergeTestDoc("Hidden", mergeHiddenTask(false, "Renamed"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("one-sided hide conflicted: %+v", result.Conflicts)
	}
	merged := result.Merged.Tasks[0]
	if !merged.Hidden || merged.Title != "Renamed" {
		t.Fatalf("merged task = hidden %v title %q, want both edits", merged.Hidden, merged.Title)
	}
}

// TestMergeAdoptsAOneSidedUnhide covers the other direction, so neither value is
// sticky.
func TestMergeAdoptsAOneSidedUnhide(t *testing.T) {
	base := mergeTestDoc("Hidden", mergeHiddenTask(true, "Task"))
	local := mergeTestDoc("Hidden", mergeHiddenTask(true, "Task"))
	current := mergeTestDoc("Hidden", mergeHiddenTask(false, "Task"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() || result.Merged.Tasks[0].Hidden {
		t.Fatalf("unhide did not land: hidden %v conflicts %+v", result.Merged.Tasks[0].Hidden, result.Conflicts)
	}
}
