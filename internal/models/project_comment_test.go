package models

import (
	"strings"
	"testing"
	"time"
)

const (
	projectCommentTestIDA = "11111111-2222-4333-8444-555555555555"
	projectCommentTestIDB = "66666666-7777-4888-8999-aaaaaaaaaaaa"
)

func projectCommentAt(id, text string, offset time.Duration) *ProjectComment {
	return &ProjectComment{
		ID:        id,
		Text:      text,
		CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC).Add(offset),
	}
}

// TestValidateProjectCommentRefusesUnstorableValues pins the one gate every
// surface goes through, so no writer can accept a comment the canonical
// document would refuse.
func TestValidateProjectCommentRefusesUnstorableValues(t *testing.T) {
	stamp := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for name, comment := range map[string]*ProjectComment{
		"empty id":        {ID: "", Text: "note", CreatedAt: stamp},
		"non-uuid id":     {ID: "comment-1", Text: "note", CreatedAt: stamp},
		"uppercase id":    {ID: strings.ToUpper(projectCommentTestIDB), Text: "note", CreatedAt: stamp},
		"braced id":       {ID: "{" + projectCommentTestIDA + "}", Text: "note", CreatedAt: stamp},
		"empty text":      {ID: projectCommentTestIDA, Text: "", CreatedAt: stamp},
		"whitespace text": {ID: projectCommentTestIDA, Text: " \n\t ", CreatedAt: stamp},
		"zero instant":    {ID: projectCommentTestIDA, Text: "note"},
	} {
		if err := ValidateProjectComment(comment.ID, comment.Text, comment.CreatedAt); err == nil {
			t.Fatalf("%s: validation accepted an unstorable comment", name)
		}
	}
	if err := ValidateProjectComment(projectCommentTestIDA, "note", stamp); err != nil {
		t.Fatalf("validation refused a storable comment: %v", err)
	}
}

// TestValidateProjectCommentsRefusesDuplicateIdentity is the collection-level
// invariant the merge depends on: two comments answering to one id are
// indistinguishable to every identity-keyed reader.
func TestValidateProjectCommentsRefusesDuplicateIdentity(t *testing.T) {
	comments := []*ProjectComment{
		projectCommentAt(projectCommentTestIDA, "one", 0),
		projectCommentAt(projectCommentTestIDA, "two", time.Hour),
	}
	err := ValidateProjectComments(comments)
	if err == nil || !strings.Contains(err.Error(), "duplicate project comment id") {
		t.Fatalf("validation error = %v, want a duplicate-id refusal", err)
	}
	if err := ValidateProjectComments([]*ProjectComment{nil}); err == nil {
		t.Fatal("validation accepted a nil comment")
	}
	if err := ValidateProjectComments(nil); err != nil {
		t.Fatalf("validation refused an empty collection: %v", err)
	}
}

// TestCloneProjectCommentsDetachesEveryValue is what a snapshot, a rollback
// document, and a live project rely on to never share one comment object.
func TestCloneProjectCommentsDetachesEveryValue(t *testing.T) {
	source := []*ProjectComment{projectCommentAt(projectCommentTestIDA, "original", 0)}
	cloned := CloneProjectComments(source)
	cloned[0].Text = "edited through the clone"
	if source[0].Text != "original" {
		t.Fatalf("clone aliased the source comment: %q", source[0].Text)
	}
	if CloneProjectComments(nil) != nil {
		t.Fatal("cloning an empty collection allocated one")
	}
}

// TestSortProjectCommentsIsTotalAndDerived proves the order comes only from
// stored values, so two machines holding the same set write the same bytes.
func TestSortProjectCommentsIsTotalAndDerived(t *testing.T) {
	comments := []*ProjectComment{
		projectCommentAt(projectCommentTestIDB, "later", time.Hour),
		projectCommentAt(projectCommentTestIDB, "tied b", 0),
		projectCommentAt(projectCommentTestIDA, "tied a", 0),
	}
	SortProjectComments(comments)
	if comments[0].Text != "tied a" || comments[1].Text != "tied b" || comments[2].Text != "later" {
		t.Fatalf("comments are not in canonical order: %+v", comments)
	}
}

// TestProjectCommentEqualComparesInstants keeps the same moment spelled in two
// zones one value, which is what stops a merge from reporting a conflict two
// machines do not actually have.
func TestProjectCommentEqualComparesInstants(t *testing.T) {
	zone := time.FixedZone("test-plus-two", 2*60*60)
	left := &ProjectComment{ID: projectCommentTestIDA, Text: "note", CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)}
	right := &ProjectComment{ID: projectCommentTestIDA, Text: "note", CreatedAt: time.Date(2026, 8, 1, 14, 0, 0, 0, zone)}
	if !ProjectCommentEqual(left, right) {
		t.Fatal("one instant spelled in two zones compared unequal")
	}
	if ProjectCommentEqual(left, nil) {
		t.Fatal("a stored comment compared equal to an absent one")
	}
	if !ProjectCommentEqual(nil, nil) {
		t.Fatal("two absent comments compared unequal")
	}
	right.Text = "edited"
	if ProjectCommentEqual(left, right) {
		t.Fatal("an edited comment compared equal")
	}
	right.Text = "note"
	right.ID = projectCommentTestIDB
	if ProjectCommentEqual(left, right) {
		t.Fatal("two identities compared equal")
	}
}

// TestAuditProjectRefusesAnInvalidCommentCollection keeps the comment audit
// beside the task graph, where the caller still holds the project, rather than
// at a render already inside a transaction.
func TestAuditProjectRefusesAnInvalidCommentCollection(t *testing.T) {
	project := &Project{
		Name: "Comments",
		Comments: []*ProjectComment{
			projectCommentAt(projectCommentTestIDA, "one", 0),
			projectCommentAt(projectCommentTestIDA, "two", time.Hour),
		},
	}
	if err := AuditProject(project); err == nil {
		t.Fatal("AuditProject accepted duplicate comment identities")
	}
	project.Comments = []*ProjectComment{projectCommentAt(projectCommentTestIDA, "one", 0)}
	if err := AuditProject(project); err != nil {
		t.Fatalf("AuditProject refused a valid collection: %v", err)
	}
}
