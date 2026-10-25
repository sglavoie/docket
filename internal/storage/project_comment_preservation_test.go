package storage

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// This file is the preservation contract for project comments across the live
// paths that rewrite tasks.md. Every one of them used to render a project's
// name and task list; a path that still did would silently drop the collection
// on the next save, which is the one failure the whole feature has to be immune
// to.

func preservationComment(id, text string, offset time.Duration) *models.ProjectComment {
	return &models.ProjectComment{
		ID:        id,
		Text:      text,
		CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC).Add(offset),
	}
}

func preservationComments() []*models.ProjectComment {
	return []*models.ProjectComment{
		preservationComment(v17TestCommentIDA, "the earlier note", 0),
		preservationComment(v17TestCommentIDB, "the later note", time.Hour),
	}
}

// preservationProject writes a project carrying comments to disk and returns it
// beside the bytes on disk, so a caller can prove a later rewrite kept them.
func preservationProject(t *testing.T) (*models.Project, string) {
	t.Helper()
	dataDir := t.TempDir()
	project := newCommitTestProject(t, dataDir, "comments", "Carry the collection")
	project.Comments = preservationComments()
	content, err := WriteProjectMarkdown(project)
	if err != nil {
		t.Fatalf("WriteProjectMarkdown: %v", err)
	}
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(TasksFilePath(project.Path), []byte(content), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return project, content
}

// sameProjectComments compares two collections element by element. The merge
// itself compares one comment at a time, so this lives with the tests that need
// a whole-collection assertion rather than in the model beside the value.
func sameProjectComments(left, right []*models.ProjectComment) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !models.ProjectCommentEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func assertPreservedComments(t *testing.T, where string, got []*models.ProjectComment) {
	t.Helper()
	if !sameProjectComments(got, preservationComments()) {
		t.Fatalf("%s did not preserve the project comments: %+v", where, got)
	}
}

// TestProjectCommentsSurviveTheLiveWriterAndReader is the base case every other
// one below depends on: the live writer renders the collection and the live
// reader gives it back.
func TestProjectCommentsSurviveTheLiveWriterAndReader(t *testing.T) {
	_, content := preservationProject(t)
	if !strings.Contains(content, "the earlier note") {
		t.Fatalf("live writer dropped the comment text:\n%s", content)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	assertPreservedComments(t, "the live reader", parsed.Comments)
}

// TestProjectCommentsChangeTheDocumentRevision proves the collection is inside
// the hash rather than beside it. A comment that did not move the revision
// would let a stale writer overwrite one silently.
func TestProjectCommentsChangeTheDocumentRevision(t *testing.T) {
	bare := ToProjectDocument(&models.Project{Name: "Comments", Tasks: []*models.Task{mergeTestTask("task-1", "Only")}})
	withComments := ToProjectDocument(&models.Project{
		Name: "Comments", Tasks: []*models.Task{mergeTestTask("task-1", "Only")},
		Comments: preservationComments(),
	})
	if bare.Revision == withComments.Revision {
		t.Fatal("adding a project comment did not change the document revision")
	}
	edited := ToProjectDocument(&models.Project{
		Name: "Comments", Tasks: []*models.Task{mergeTestTask("task-1", "Only")},
		Comments: []*models.ProjectComment{
			preservationComment(v17TestCommentIDA, "the earlier note, edited", 0),
			preservationComment(v17TestCommentIDB, "the later note", time.Hour),
		},
	})
	if edited.Revision == withComments.Revision {
		t.Fatal("editing a project comment did not change the document revision")
	}
}

// TestProjectCommentsSurviveLoadPaths covers the three readers a mutation goes
// through: the plain load, the same-read model/document pair, and the document
// load used for a rewrite base.
func TestProjectCommentsSurviveLoadPaths(t *testing.T) {
	project, _ := preservationProject(t)

	plain, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatal(err)
	}
	plain.Slug = project.Slug
	if loadErr := LoadProject(plain); loadErr != nil {
		t.Fatalf("LoadProject: %v", loadErr)
	}
	assertPreservedComments(t, "LoadProject", plain.Comments)

	paired, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatal(err)
	}
	paired.Slug = project.Slug
	document, err := LoadProjectWithDocument(paired)
	if err != nil {
		t.Fatalf("LoadProjectWithDocument: %v", err)
	}
	assertPreservedComments(t, "the same-read model", paired.Comments)
	assertPreservedComments(t, "the same-read base document", document.Comments)
	// The document is the immutable merge base, so editing the model must not
	// reach it.
	paired.Comments[0].Text = "edited through the model"
	assertPreservedComments(t, "the base document after a model edit", document.Comments)

	loaded, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	assertPreservedComments(t, "LoadProjectDocument", loaded.Comments)
}

// TestMissingProjectDocumentStartsWithNoComments pins the initialization
// branch: an absent file is an empty project, not an error and not an
// invented collection.
func TestMissingProjectDocumentStartsWithNoComments(t *testing.T) {
	dataDir := t.TempDir()
	project := newCommitTestProject(t, dataDir, "absent", "Nothing on disk")
	project.Comments = preservationComments()
	document, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	if len(document.Comments) != 0 {
		t.Fatalf("missing document invented comments: %+v", document.Comments)
	}
	if err := LoadProject(project); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(project.Comments) != 0 {
		t.Fatalf("missing document left stale comments on the project: %+v", project.Comments)
	}
}

// TestProjectCommentsSurviveACommitAndReload is the production rewrite path: a
// task edit committed through the repository must not drop the project's own
// collection.
func TestProjectCommentsSurviveACommitAndReload(t *testing.T) {
	project, _ := preservationProject(t)
	service := NewCommitService(TasksFilePath(t.TempDir()), []string{project.Path})
	base, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	project.Tasks[0].Title = "Edited beside the comments"
	if _, commitErr := service.Commit(ChangeSet{
		Project: project, ExpectedRevision: base.Revision, Base: base, UndoAction: "Edit Task",
	}); commitErr != nil {
		t.Fatalf("Commit: %v", commitErr)
	}
	reloaded, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.Slug = project.Slug
	if err := LoadProject(reloaded); err != nil {
		t.Fatalf("LoadProject after commit: %v", err)
	}
	if reloaded.Tasks[0].Title != "Edited beside the comments" {
		t.Fatalf("task edit did not commit: %q", reloaded.Tasks[0].Title)
	}
	assertPreservedComments(t, "a task-edit commit", reloaded.Comments)
}

// TestCanonicalProjectDocumentKeepsComments covers the canonicalization every
// commit renders through, which is also what a rollback document is rebuilt
// from.
func TestCanonicalProjectDocumentKeepsComments(t *testing.T) {
	project := &models.Project{
		Name: "Comments", Tasks: []*models.Task{mergeTestTask("task-1", "Only")},
		Comments: preservationComments(),
	}
	canonical, err := CanonicalProjectDocument(project, nil)
	if err != nil {
		t.Fatalf("CanonicalProjectDocument: %v", err)
	}
	assertPreservedComments(t, "canonicalization", canonical.Comments)
}

// TestToProjectDocumentDetachesComments is what keeps a rollback honest: the
// base document a mutation is measured against must not be editable through the
// live project it was taken from.
func TestToProjectDocumentDetachesComments(t *testing.T) {
	project := &models.Project{
		Name: "Comments", Tasks: []*models.Task{mergeTestTask("task-1", "Only")},
		Comments: preservationComments(),
	}
	document := ToProjectDocument(project)
	project.Comments[0].Text = "edited after the snapshot"
	assertPreservedComments(t, "a detached document", document.Comments)
}

// TestProjectCommentsSurviveConflictReconstruction covers the recovery path: a
// bundle records the documents, and resolving one rebuilds a project that still
// carries the winning side's collection.
func TestProjectCommentsSurviveConflictReconstruction(t *testing.T) {
	project, content := preservationProject(t)
	document, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	store := NewConflictStore(t.TempDir())
	id, err := RecordMergeConflict(store, project, document, document, document,
		[]MergeConflict{{Kind: MergeConflictProjectMetadata, Field: "comments", Detail: "test"}},
		"Edit Task", project.Tasks[0].Title)
	if err != nil {
		t.Fatalf("RecordMergeConflict: %v", err)
	}
	bundle, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load bundle: %v", err)
	}
	assertPreservedComments(t, "the stored local document", bundle.Local.Comments)
	assertPreservedComments(t, "the stored base document", bundle.Base.Comments)
	rebuilt := projectFromDocument(project, bundle.Local)
	assertPreservedComments(t, "a project rebuilt from a bundle", rebuilt.Comments)
	rebuilt.Comments[0].Text = "edited through the rebuilt project"
	assertPreservedComments(t, "the bundle after a rebuilt-project edit", bundle.Local.Comments)
}

// TestAuditProjectRefusesAnUnwritableCommentCollection keeps the loaded project
// as strict as the canonical writer, so a merge producing a collection the
// writer would refuse is a recorded conflict rather than a commit-time render
// failure naming no bundle.
func TestAuditProjectRefusesAnUnwritableCommentCollection(t *testing.T) {
	document := mergeTestDoc("Comments", mergeTestTask("task-1", "Only"))
	document.Comments = []*models.ProjectComment{
		preservationComment(v17TestCommentIDA, "one", 0),
		preservationComment(v17TestCommentIDA, "two", time.Hour),
	}
	conflicts := auditMergedDocument(document)
	if len(conflicts) == 0 {
		t.Fatal("the merged-document audit accepted duplicate comment identities")
	}
	if !strings.Contains(conflicts[0].Detail, "duplicate project comment id") {
		t.Fatalf("audit conflict does not name the duplicate: %+v", conflicts[0])
	}
}

// TestMergeProjectDocumentsDetachesMergedComments keeps the merge result from
// aliasing the side it adopted, so committing it cannot edit an input document.
func TestMergeProjectDocumentsDetachesMergedComments(t *testing.T) {
	base := mergeTestDoc("Comments", mergeTestTask("task-1", "Only"))
	local := mergeTestDoc("Comments", mergeTestTask("task-1", "Only"))
	local.Comments = preservationComments()
	current := mergeTestDoc("Comments", mergeTestTask("task-1", "Only"))

	result, err := MergeProjectDocuments(base, local, current)
	if err != nil {
		t.Fatalf("MergeProjectDocuments: %v", err)
	}
	if result.HasConflicts() || result.Merged == nil {
		t.Fatalf("a one-sided comment addition conflicted: %+v", result.Conflicts)
	}
	assertPreservedComments(t, "the merged document", result.Merged.Comments)
	result.Merged.Comments[0].Text = "edited through the merge result"
	assertPreservedComments(t, "the local input after a merged edit", local.Comments)
}
