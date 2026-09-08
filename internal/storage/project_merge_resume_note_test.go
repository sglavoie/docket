package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestMergeableTaskFieldsIncludesResumeNote proves the reflective inventory
// admitted the new persisted field on its own. Nothing enumerates merge fields
// by hand, so this is what keeps a resume note from being silently dropped by a
// concurrent commit.
func TestMergeableTaskFieldsIncludesResumeNote(t *testing.T) {
	for _, field := range mergeableTaskFields {
		if field.jsonName == "resume_note" {
			return
		}
	}
	t.Fatal("merge field inventory does not include resume_note")
}

func mergeResumeNoteTask(note string) *models.Task {
	task := mergeTestTask("t1", "Task")
	task.ResumeNote = note
	return task
}

// TestMergeAdoptsAOneSidedResumeNoteEdit is the ordinary case: one side wrote
// the note, the other did not touch it, so the note lands without a conflict.
func TestMergeAdoptsAOneSidedResumeNoteEdit(t *testing.T) {
	base := mergeTestDoc("Resume", mergeResumeNoteTask(""))
	local := mergeTestDoc("Resume", mergeResumeNoteTask("stopped at the failing case"))
	current := mergeTestDoc("Resume", mergeResumeNoteTask(""))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("one-sided resume note edit conflicted: %+v", result.Conflicts)
	}
	if result.Merged.Tasks[0].ResumeNote != "stopped at the failing case" {
		t.Fatalf("merged resume note = %q", result.Merged.Tasks[0].ResumeNote)
	}
}

// TestMergeCoalescesIdenticalResumeNoteEdits keeps a duplicated edit from being
// reported as divergence.
func TestMergeCoalescesIdenticalResumeNoteEdits(t *testing.T) {
	base := mergeTestDoc("Resume", mergeResumeNoteTask(""))
	local := mergeTestDoc("Resume", mergeResumeNoteTask("same note"))
	current := mergeTestDoc("Resume", mergeResumeNoteTask("same note"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("identical resume note edits conflicted: %+v", result.Conflicts)
	}
	if result.Merged.Tasks[0].ResumeNote != "same note" {
		t.Fatalf("merged resume note = %q", result.Merged.Tasks[0].ResumeNote)
	}
}

// TestMergeReportsDivergentResumeNotesAsAFieldConflict pins the note as one
// value rather than something two machines can each contribute a piece of: the
// note is a person's sentence, so a merged half of one would be worse than
// asking.
func TestMergeReportsDivergentResumeNotesAsAFieldConflict(t *testing.T) {
	base := mergeTestDoc("Resume", mergeResumeNoteTask(""))
	local := mergeTestDoc("Resume", mergeResumeNoteTask("left note"))
	current := mergeTestDoc("Resume", mergeResumeNoteTask("right note"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.Merged != nil {
		t.Fatal("a conflicting merge produced a document")
	}
	for _, conflict := range result.Conflicts {
		if conflict.Field == "resume_note" {
			if conflict.Kind != MergeConflictFieldEdit {
				t.Fatalf("resume note conflict kind = %q, want %q", conflict.Kind, MergeConflictFieldEdit)
			}
			return
		}
	}
	t.Fatalf("divergent resume notes produced no resume_note conflict: %+v", result.Conflicts)
}

// TestMergeAdoptsAOneSidedResumeNoteClear keeps clearing a note a real edit: the
// empty string is the only spelling of "no note", so a side that erased one must
// not read as a side that left it alone.
func TestMergeAdoptsAOneSidedResumeNoteClear(t *testing.T) {
	base := mergeTestDoc("Resume", mergeResumeNoteTask("an old note"))
	local := mergeTestDoc("Resume", mergeResumeNoteTask(""))
	current := mergeTestDoc("Resume", mergeResumeNoteTask("an old note"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("one-sided resume note clear conflicted: %+v", result.Conflicts)
	}
	if result.Merged.Tasks[0].ResumeNote != "" {
		t.Fatalf("merged resume note = %q, want it cleared", result.Merged.Tasks[0].ResumeNote)
	}
}
