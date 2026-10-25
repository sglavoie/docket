package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// This file is the contract for the identity-keyed project-comment merge. The
// collection is the one place two machines can write prose concurrently without
// either seeing the other, so the cases below are stated as the outcomes a user
// is entitled to rather than as the branches the implementation happens to have:
// independent writes survive, agreement coalesces, disagreement is reported with
// enough evidence to resolve, and nothing partial is ever written.

const (
	mergeCommentIDA = "aaaaaaaa-1111-4111-8111-111111111111"
	mergeCommentIDB = "bbbbbbbb-2222-4222-8222-222222222222"
	mergeCommentIDC = "cccccccc-3333-4333-8333-333333333333"
	mergeCommentIDD = "dddddddd-4444-4444-8444-444444444444"
)

var mergeCommentEpoch = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

func mergeComment(id, text string, offset time.Duration) *models.ProjectComment {
	return &models.ProjectComment{ID: id, Text: text, CreatedAt: mergeCommentEpoch.Add(offset)}
}

// mergeCommentBase is the collection every three-way fixture below starts from:
// two comments an hour apart, already in canonical order.
func mergeCommentBase() []*models.ProjectComment {
	return []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
	}
}

// commentIDs names what a merge produced, which is what most assertions below
// actually care about; the bodies are checked where the case is about a body.
func commentIDs(comments []*models.ProjectComment) []string {
	ids := make([]string, 0, len(comments))
	for _, comment := range comments {
		ids = append(ids, comment.ID)
	}
	return ids
}

func commentTextByID(t *testing.T, comments []*models.ProjectComment, id string) string {
	t.Helper()
	for _, comment := range comments {
		if comment.ID == id {
			return comment.Text
		}
	}
	t.Fatalf("comment %s is missing from %v", id, commentIDs(comments))
	return ""
}

func assertCommentIDs(t *testing.T, got []*models.ProjectComment, want ...string) {
	t.Helper()
	ids := commentIDs(got)
	if len(ids) != len(want) {
		t.Fatalf("merged comments = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("merged comments = %v, want %v", ids, want)
		}
	}
}

// findCommentConflict locates the entry for one identity and field, which is the
// pair the record is supposed to be addressable by.
func findCommentConflict(conflicts []MergeConflict, id, field string) (MergeConflict, bool) {
	for _, conflict := range conflicts {
		if conflict.CommentID == id && conflict.Field == field {
			return conflict, true
		}
	}
	return MergeConflict{}, false
}

// TestMergeProjectCommentsKeepsIndependentWrites is the case the whole-collection
// seam could not answer: two machines that never saw each other's comment both
// keep theirs, and an edit to one comment does not contend with an edit to
// another.
func TestMergeProjectCommentsKeepsIndependentWrites(t *testing.T) {
	base := mergeCommentBase()
	local := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "the second note, rewritten locally", time.Hour),
		mergeComment(mergeCommentIDC, "added on local", 2*time.Hour),
	}
	current := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note, rewritten remotely", 0),
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
		mergeComment(mergeCommentIDD, "added on current", 3*time.Hour),
	}

	merged, conflicts := mergeProjectComments(base, local, current)
	if len(conflicts) != 0 {
		t.Fatalf("independent writes conflicted: %+v", conflicts)
	}
	assertCommentIDs(t, merged, mergeCommentIDA, mergeCommentIDB, mergeCommentIDC, mergeCommentIDD)
	if got := commentTextByID(t, merged, mergeCommentIDA); got != "the first note, rewritten remotely" {
		t.Fatalf("the remote edit was not adopted: %q", got)
	}
	if got := commentTextByID(t, merged, mergeCommentIDB); got != "the second note, rewritten locally" {
		t.Fatalf("the local edit was not adopted: %q", got)
	}
}

// TestMergeProjectCommentsCoalescesIdenticalChanges keeps two machines that
// arrived at the same value from producing a conflict they cannot act on: there
// is nothing to choose between.
func TestMergeProjectCommentsCoalescesIdenticalChanges(t *testing.T) {
	base := mergeCommentBase()
	same := func() []*models.ProjectComment {
		return []*models.ProjectComment{
			mergeComment(mergeCommentIDA, "the first note, rewritten", 0),
			mergeComment(mergeCommentIDB, "the second note", time.Hour),
			mergeComment(mergeCommentIDC, "added identically", 2*time.Hour),
		}
	}

	merged, conflicts := mergeProjectComments(base, same(), same())
	if len(conflicts) != 0 {
		t.Fatalf("identical concurrent changes conflicted: %+v", conflicts)
	}
	assertCommentIDs(t, merged, mergeCommentIDA, mergeCommentIDB, mergeCommentIDC)
	if got := commentTextByID(t, merged, mergeCommentIDA); got != "the first note, rewritten" {
		t.Fatalf("the coalesced edit is not the value both sides wrote: %q", got)
	}
}

// TestMergeProjectCommentsResolvesDeletion covers the two deletions that are not
// races: one side removing a comment the other left alone, and both sides
// removing the same one.
func TestMergeProjectCommentsResolvesDeletion(t *testing.T) {
	base := mergeCommentBase()
	withoutB := []*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)}

	merged, conflicts := mergeProjectComments(base, withoutB, mergeCommentBase())
	if len(conflicts) != 0 {
		t.Fatalf("a one-sided deletion conflicted: %+v", conflicts)
	}
	assertCommentIDs(t, merged, mergeCommentIDA)

	merged, conflicts = mergeProjectComments(base, mergeCommentBase(), withoutB)
	if len(conflicts) != 0 {
		t.Fatalf("a remote-only deletion conflicted: %+v", conflicts)
	}
	assertCommentIDs(t, merged, mergeCommentIDA)

	merged, conflicts = mergeProjectComments(base, withoutB, withoutB)
	if len(conflicts) != 0 {
		t.Fatalf("a deletion on both sides conflicted: %+v", conflicts)
	}
	assertCommentIDs(t, merged, mergeCommentIDA)
}

// TestMergeProjectCommentsReportsEveryDivergence is the completeness claim: a
// merge does not stop at the first disagreement, every entry names the identity
// it belongs to and the field that diverged, and each side is described. A
// record that named only the first conflict would send the operator through the
// resolve/retry loop once per comment.
func TestMergeProjectCommentsReportsEveryDivergence(t *testing.T) {
	base := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
		mergeComment(mergeCommentIDC, "the third note", 2*time.Hour),
	}
	local := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "rewritten on local", 0),
		mergeComment(mergeCommentIDC, "the third note", 2*time.Hour),
		mergeComment(mergeCommentIDD, "added on local", 3*time.Hour),
	}
	current := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "rewritten on current", 0),
		mergeComment(mergeCommentIDB, "rewritten on current too", time.Hour),
		mergeComment(mergeCommentIDC, "the third note", 2*time.Hour),
		mergeComment(mergeCommentIDD, "added on current", 3*time.Hour),
	}

	_, conflicts := mergeProjectComments(base, local, current)
	if len(conflicts) != 3 {
		t.Fatalf("expected one conflict per diverging identity, got %d: %+v", len(conflicts), conflicts)
	}

	textEdit, ok := findCommentConflict(conflicts, mergeCommentIDA, projectCommentTextField)
	if !ok {
		t.Fatalf("divergent text edits are not reported: %+v", conflicts)
	}
	if textEdit.Detail != "project comment text changed on both sides" {
		t.Fatalf("text conflict detail = %q", textEdit.Detail)
	}

	deleteEdit, ok := findCommentConflict(conflicts, mergeCommentIDB, projectCommentField)
	if !ok {
		t.Fatalf("the edit/delete race is not reported: %+v", conflicts)
	}
	if deleteEdit.Local != "absent" {
		t.Fatalf("the deleting side is not described as absent: %+v", deleteEdit)
	}
	if !strings.Contains(deleteEdit.Detail, "deleted on local") {
		t.Fatalf("the edit/delete race does not name which side deleted: %q", deleteEdit.Detail)
	}

	addition, ok := findCommentConflict(conflicts, mergeCommentIDD, projectCommentField)
	if !ok {
		t.Fatalf("a divergent addition under one identity is not reported: %+v", conflicts)
	}
	if addition.Base != "absent" {
		t.Fatalf("an addition's base is not described as absent: %+v", addition)
	}

	// Every entry is project metadata located by identity, never a task, and every
	// entry describes all three sides so a bundle can be read without re-running
	// the merge.
	for _, conflict := range conflicts {
		if conflict.Kind != MergeConflictProjectMetadata {
			t.Fatalf("comment conflict kind = %q, want project metadata: %+v", conflict.Kind, conflict)
		}
		if conflict.TaskTitle != "" {
			t.Fatalf("comment conflict borrowed a task title: %+v", conflict)
		}
		if conflict.CommentID == "" || conflict.Detail == "" {
			t.Fatalf("comment conflict is not addressable: %+v", conflict)
		}
		if conflict.Base == "" || conflict.Local == "" || conflict.Current == "" {
			t.Fatalf("comment conflict does not describe all three sides: %+v", conflict)
		}
	}
}

// TestMergeProjectCommentConflictsNameNoProse keeps the bodies in the stored
// documents. The bundle already holds all three; a summary that inlined the text
// would be the only place a comment was duplicated, and a digest answers the one
// question a summary has to answer — whether two sides hold the same body.
func TestMergeProjectCommentConflictsNameNoProse(t *testing.T) {
	base := []*models.ProjectComment{mergeComment(mergeCommentIDA, "the original body", 0)}
	local := []*models.ProjectComment{mergeComment(mergeCommentIDA, "the local body", 0)}
	current := []*models.ProjectComment{mergeComment(mergeCommentIDA, "the remote body", 0)}

	_, conflicts := mergeProjectComments(base, local, current)
	if len(conflicts) != 1 {
		t.Fatalf("expected one conflict, got %+v", conflicts)
	}
	conflict := conflicts[0]
	for _, body := range []string{"the original body", "the local body", "the remote body"} {
		for _, side := range []string{conflict.Base, conflict.Local, conflict.Current} {
			if strings.Contains(side, body) {
				t.Fatalf("conflict summary %q reproduces a comment body", side)
			}
		}
	}
	if conflict.Local == conflict.Current {
		t.Fatalf("two different bodies produced one summary: %q", conflict.Local)
	}
	if !strings.Contains(conflict.Local, mergeCommentEpoch.Format(time.RFC3339Nano)) {
		t.Fatalf("conflict summary does not name the creation instant: %q", conflict.Local)
	}
}

// TestMergeProjectCommentsRefusesAMovedCreationTimestamp holds the instant as
// identity metadata. It is half of the canonical order and no mutation writes
// it, so a side that moved it is reported rather than adopted — including when
// both sides moved it the same way, because agreement between two machines is
// not permission for the value to change.
func TestMergeProjectCommentsRefusesAMovedCreationTimestamp(t *testing.T) {
	base := []*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)}
	moved := []*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", time.Hour)}

	for _, tc := range []struct {
		name           string
		local, current []*models.ProjectComment
	}{
		{"local moved it", moved, base},
		{"current moved it", base, moved},
		{"both moved it the same way", moved, moved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merged, conflicts := mergeProjectComments(base, tc.local, tc.current)
			conflict, ok := findCommentConflict(conflicts, mergeCommentIDA, projectCommentCreatedAtField)
			if !ok {
				t.Fatalf("a moved creation timestamp was adopted: merged=%+v conflicts=%+v", merged, conflicts)
			}
			if conflict.Detail != "a project comment's creation timestamp is immutable" {
				t.Fatalf("conflict detail = %q", conflict.Detail)
			}
			if len(merged) != 0 {
				t.Fatalf("a refused comment was still merged: %+v", merged)
			}
		})
	}
}

// TestMergeProjectCommentsOrderIsCanonicalAndSideIndependent is the determinism
// claim. Two machines merging the same three collections must write the same
// bytes, so the result may not depend on the order the inputs happened to be in
// or on which side the caller called local.
func TestMergeProjectCommentsOrderIsCanonicalAndSideIndependent(t *testing.T) {
	base := []*models.ProjectComment{mergeComment(mergeCommentIDB, "the second note", time.Hour)}
	// Deliberately out of canonical order: an addition older than the base entry
	// arrives after it.
	local := []*models.ProjectComment{
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
		mergeComment(mergeCommentIDA, "an older note added locally", 0),
	}
	current := []*models.ProjectComment{
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
		mergeComment(mergeCommentIDD, "a newer note added remotely", 3*time.Hour),
	}

	merged, conflicts := mergeProjectComments(base, local, current)
	if len(conflicts) != 0 {
		t.Fatalf("unordered independent additions conflicted: %+v", conflicts)
	}
	assertCommentIDs(t, merged, mergeCommentIDA, mergeCommentIDB, mergeCommentIDD)

	mirrored, mirroredConflicts := mergeProjectComments(base, current, local)
	if len(mirroredConflicts) != 0 {
		t.Fatalf("the mirrored merge conflicted: %+v", mirroredConflicts)
	}
	if !sameProjectComments(merged, mirrored) {
		t.Fatalf("swapping local and current changed the result: %v vs %v",
			commentIDs(merged), commentIDs(mirrored))
	}

	// The conflict list is ordered by the same canonical key, so two machines
	// reading one bundle read the entries in one order.
	divergentLocal := []*models.ProjectComment{
		mergeComment(mergeCommentIDB, "local rewrote the second", time.Hour),
		mergeComment(mergeCommentIDA, "local kept the first", 0),
	}
	divergentCurrent := []*models.ProjectComment{
		mergeComment(mergeCommentIDB, "current rewrote the second", time.Hour),
		mergeComment(mergeCommentIDA, "current kept the first", 0),
	}
	orderedBase := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
	}
	_, ordered := mergeProjectComments(orderedBase, divergentLocal, divergentCurrent)
	if len(ordered) != 2 {
		t.Fatalf("expected both identities to conflict, got %+v", ordered)
	}
	if ordered[0].CommentID != mergeCommentIDA || ordered[1].CommentID != mergeCommentIDB {
		t.Fatalf("conflicts are not in canonical order: %s then %s", ordered[0].CommentID, ordered[1].CommentID)
	}
}

// TestMergeProjectCommentsDoesNotTouchItsInputs proves the merge is a pure
// reader of three documents and that its result is detached from all of them.
// The base is a caller's immutable snapshot and the current document is the
// remote's live state; a merge that wrote through either would corrupt the
// comparison the next commit is measured against.
func TestMergeProjectCommentsDoesNotTouchItsInputs(t *testing.T) {
	base := mergeCommentBase()
	local := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "rewritten locally", time.Hour),
		mergeComment(mergeCommentIDC, "added on local", 2*time.Hour),
	}
	current := []*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "the second note", time.Hour),
		mergeComment(mergeCommentIDD, "added on current", 3*time.Hour),
	}
	baseBefore, localBefore, currentBefore := mergeCommentBase(), append([]*models.ProjectComment(nil), local...), append([]*models.ProjectComment(nil), current...)

	merged, conflicts := mergeProjectComments(base, local, current)
	if len(conflicts) != 0 {
		t.Fatalf("independent writes conflicted: %+v", conflicts)
	}
	if !sameProjectComments(base, baseBefore) {
		t.Fatalf("the merge edited its base: %+v", base)
	}
	if !sameProjectComments(local, localBefore) || !sameProjectComments(current, currentBefore) {
		t.Fatal("the merge edited one of the sides it read")
	}

	// Every adopted value is detached, whichever branch produced it: the comment
	// merged field by field, the one adopted whole from local, and the one adopted
	// whole from current.
	for _, comment := range merged {
		comment.Text = "edited through the merge result"
	}
	if !sameProjectComments(base, baseBefore) || !sameProjectComments(local, localBefore) ||
		!sameProjectComments(current, currentBefore) {
		t.Fatal("editing the merged collection reached back into an input document")
	}
}

// --- document-level merges ------------------------------------------------

// commentTestDoc builds a project document carrying the given tasks and
// comments, with its canonical revision computed.
func commentTestDoc(comments []*models.ProjectComment, tasks ...*models.Task) *ProjectDocument {
	doc := mergeTestDoc("Comments", tasks...)
	doc.Comments = comments
	doc.Revision = doc.ComputeRevision()
	return doc
}

// TestMergeProjectDocumentsMergesCommentsBesideUnrelatedTaskWork proves the two
// halves of a document do not contend: a comment written on one side lands
// beside a task edit and a task creation written on the other, and neither
// component is discarded.
func TestMergeProjectDocumentsMergesCommentsBesideUnrelatedTaskWork(t *testing.T) {
	anchor := mergeTestTask("t1", "the anchor")
	base := commentTestDoc([]*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)}, anchor)

	local := commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "added while editing a task", time.Hour),
	}, mergeTestTask("t1", "the anchor, retitled locally"))

	created := mergeTestTask("t2", "created on current")
	current := commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDC, "added while creating a task", 2*time.Hour),
	}, mergeTestTask("t1", "the anchor"), created)

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() {
		t.Fatalf("comment writes contended with unrelated task work: %+v", result.Conflicts)
	}
	assertCommentIDs(t, result.Merged.Comments, mergeCommentIDA, mergeCommentIDB, mergeCommentIDC)
	if len(result.Merged.Tasks) != 2 {
		t.Fatalf("merged tasks = %d, want the anchor and the creation", len(result.Merged.Tasks))
	}
	if result.Merged.Tasks[0].Title != "the anchor, retitled locally" {
		t.Fatalf("the local task edit was discarded: %q", result.Merged.Tasks[0].Title)
	}
	if result.Merged.Tasks[1].ID != "t2" {
		t.Fatalf("the remote task creation was discarded: %+v", commentIDs(result.Merged.Comments))
	}
}

// TestMergeProjectDocumentsAbortsWholeOnACommentConflict is the all-or-nothing
// boundary at the pure-merge level: a comment that cannot be reconciled costs
// the whole document, including the task edit that would otherwise have merged
// cleanly beside it.
func TestMergeProjectDocumentsAbortsWholeOnACommentConflict(t *testing.T) {
	anchor := mergeTestTask("t1", "the anchor")
	base := commentTestDoc([]*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)}, anchor)
	local := commentTestDoc([]*models.ProjectComment{mergeComment(mergeCommentIDA, "rewritten locally", 0)},
		mergeTestTask("t1", "the anchor, retitled locally"))
	current := commentTestDoc([]*models.ProjectComment{mergeComment(mergeCommentIDA, "rewritten remotely", 0)}, anchor)

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.Merged != nil {
		t.Fatalf("a comment conflict still produced a merged document: %+v", result.Merged)
	}
	if _, ok := findCommentConflict(result.Conflicts, mergeCommentIDA, projectCommentTextField); !ok {
		t.Fatalf("the document merge did not carry the comment conflict: %+v", result.Conflicts)
	}
}
