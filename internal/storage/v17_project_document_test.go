package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/storage/format"
)

const (
	v17TestCommentIDA = "11111111-2222-4333-8444-555555555555"
	v17TestCommentIDB = "66666666-7777-4888-8999-aaaaaaaaaaaa"
)

func v17ProjectTestDocument(t *testing.T) V17ProjectDocument {
	t.Helper()
	record, err := currentTaskRecordFromModel(mergeTestTask("task-1", "Lifted"))
	if err != nil {
		t.Fatal(err)
	}
	return V17ProjectDocument{ProjectName: "Comments", Tasks: []format.V16TaskRecord{record.V16TaskRecord}}
}

func v17TestComment(id, text string, offset time.Duration) format.ProjectCommentRecord {
	return format.ProjectCommentRecord{
		ID:        id,
		Text:      text,
		CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC).Add(offset),
	}
}

// TestV17ProjectDocumentLiftFromV16AddsNoComments is what makes the schema-19
// migration a marker change: lifting a document must leave the project with no
// comments, and no comments writes no block.
func TestV17ProjectDocumentLiftFromV16AddsNoComments(t *testing.T) {
	source := V16ProjectDocument{ProjectName: "Comments", Tasks: v17ProjectTestDocument(t).Tasks}
	lifted := V17ProjectDocumentFromV16(source)
	if len(lifted.Comments) != 0 {
		t.Fatalf("lift = %+v, want no comments", lifted.Comments)
	}
	published, err := WriteV17ProjectDocument(lifted)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	if strings.Contains(published, "PROJECT-COMMENTS") {
		t.Fatalf("lifted document wrote a comment block:\n%s", published)
	}
	original, err := WriteV16ProjectDocument(source)
	if err != nil {
		t.Fatalf("WriteV16ProjectDocument: %v", err)
	}
	if strings.Replace(published, schemaMarkerV17, schemaMarkerV16, 1) != original {
		t.Fatalf("lift changed bytes beyond the marker\nv16: %s\nv17: %s", original, published)
	}
}

// TestV17ProjectDocumentRoundTripsComments covers the document level rather than
// the block: the collection has to survive the heading scan and the project-wide
// validation, not just its own codec.
func TestV17ProjectDocumentRoundTripsComments(t *testing.T) {
	document := v17ProjectTestDocument(t)
	document.Comments = []format.ProjectCommentRecord{
		v17TestComment(v17TestCommentIDA, "first note\nsecond line", 0),
		v17TestComment(v17TestCommentIDB, "later note", time.Hour),
	}
	published, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	// The block belongs above every task, because it belongs to the project.
	commentIndex := strings.Index(published, format.ProjectCommentsStartLine)
	headingIndex := strings.Index(published, "## Task:")
	if commentIndex < 0 || headingIndex < 0 || commentIndex > headingIndex {
		t.Fatalf("comment block is not above the first task heading:\n%s", published)
	}
	parsed, err := ParseV17ProjectDocument(published)
	if err != nil {
		t.Fatalf("ParseV17ProjectDocument: %v", err)
	}
	if len(parsed.Comments) != 2 {
		t.Fatalf("parsed %d comments, want 2", len(parsed.Comments))
	}
	for i, want := range document.Comments {
		got := parsed.Comments[i]
		if got.ID != want.ID || got.Text != want.Text || !got.CreatedAt.Equal(want.CreatedAt) {
			t.Fatalf("comment %d = %+v, want %+v", i, got, want)
		}
	}
	if len(parsed.Tasks) != 1 || parsed.Tasks[0].Title != document.Tasks[0].Title {
		t.Fatalf("tasks did not survive the comment block: %+v", parsed.Tasks)
	}
}

// TestV17ProjectDocumentWritesCanonicalCommentOrder proves the file, not just
// the loaded collection, is ordered: two machines holding the same set have to
// render the same bytes or every revision comparison between them is noise.
func TestV17ProjectDocumentWritesCanonicalCommentOrder(t *testing.T) {
	document := v17ProjectTestDocument(t)
	document.Comments = []format.ProjectCommentRecord{
		v17TestComment(v17TestCommentIDB, "later note", time.Hour),
		v17TestComment(v17TestCommentIDA, "first note", 0),
	}
	published, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	if strings.Index(published, v17TestCommentIDA) > strings.Index(published, v17TestCommentIDB) {
		t.Fatalf("comments were written out of canonical order:\n%s", published)
	}
	// A tie on the instant is broken by identity, so the same set still renders
	// one way.
	document.Comments[0].CreatedAt = document.Comments[1].CreatedAt
	tied, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	if strings.Index(tied, v17TestCommentIDA) > strings.Index(tied, v17TestCommentIDB) {
		t.Fatalf("tied comments were not ordered by identity:\n%s", tied)
	}
}

// TestV17ProjectDocumentNormalizesCommentZone keeps the canonical rendering — and
// therefore the revision — the same on two machines that recorded one moment in
// different places.
func TestV17ProjectDocumentNormalizesCommentZone(t *testing.T) {
	zone := time.FixedZone("test-plus-two", 2*60*60)
	document := v17ProjectTestDocument(t)
	document.Comments = []format.ProjectCommentRecord{{
		ID: v17TestCommentIDA, Text: "zoned", CreatedAt: time.Date(2026, 8, 1, 14, 0, 0, 0, zone),
	}}
	published, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	if !strings.Contains(published, "2026-08-01T12:00:00Z") {
		t.Fatalf("stored timestamp was not normalized to UTC:\n%s", published)
	}
	parsed, err := ParseV17ProjectDocument(published)
	if err != nil {
		t.Fatalf("ParseV17ProjectDocument: %v", err)
	}
	if !parsed.Comments[0].CreatedAt.Equal(document.Comments[0].CreatedAt) {
		t.Fatalf("parsed instant = %v, want %v", parsed.Comments[0].CreatedAt, document.Comments[0].CreatedAt)
	}
}

// TestV17ProjectDocumentEscapesHostileCommentProse is the whole reason the block
// reuses the prose escaping: a comment body that spells a delimiter must come
// back as text rather than closing the block or opening an entry.
func TestV17ProjectDocumentEscapesHostileCommentProse(t *testing.T) {
	hostile := strings.Join([]string{
		format.ProjectCommentsEndLine,
		"<!-- project-comment: " + v17TestCommentIDB + " 2026-08-01T12:00:00Z -->",
		"## Task: not a task",
		"# Project: not a project",
		"a & b --> c",
	}, "\n")
	document := v17ProjectTestDocument(t)
	document.Comments = []format.ProjectCommentRecord{v17TestComment(v17TestCommentIDA, hostile, 0)}
	published, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	parsed, err := ParseV17ProjectDocument(published)
	if err != nil {
		t.Fatalf("ParseV17ProjectDocument: %v", err)
	}
	if len(parsed.Comments) != 1 || parsed.Comments[0].Text != hostile {
		t.Fatalf("hostile comment changed:\n got: %+v\nwant: %q", parsed.Comments, hostile)
	}
	if len(parsed.Tasks) != 1 {
		t.Fatalf("hostile comment swallowed the task list: %+v", parsed.Tasks)
	}
}

// TestV17ProjectDocumentRefusesUnwritableComments keeps the writer as strict as
// the reader: a collection the canonical writer would not produce never reaches
// bytes.
func TestV17ProjectDocumentRefusesUnwritableComments(t *testing.T) {
	for name, comments := range map[string][]format.ProjectCommentRecord{
		"duplicate id": {
			v17TestComment(v17TestCommentIDA, "one", 0),
			v17TestComment(v17TestCommentIDA, "two", time.Hour),
		},
		"invalid id": {v17TestComment("not-a-uuid", "one", 0)},
		"upper id":   {v17TestComment(strings.ToUpper(v17TestCommentIDB), "one", 0)},
		"blank text": {v17TestComment(v17TestCommentIDA, "   \n  ", 0)},
		"zero instant": {{
			ID: v17TestCommentIDA, Text: "one",
		}},
	} {
		document := v17ProjectTestDocument(t)
		document.Comments = comments
		if _, err := WriteV17ProjectDocument(document); err == nil {
			t.Fatalf("%s: writer accepted a collection it cannot represent", name)
		}
	}
}

// TestV17ProjectDocumentRefusesMalformedCommentBlocks covers the shapes the
// canonical writer can never produce, so accepting one would mean guessing at a
// file something else wrote.
func TestV17ProjectDocumentRefusesMalformedCommentBlocks(t *testing.T) {
	header := "# Project: Comments\n" + schemaMarkerV17 + "\n"
	entry := "<!-- project-comment: " + v17TestCommentIDA + " 2026-08-01T12:00:00Z -->\nnote\n"
	block := format.ProjectCommentsStartLine + "\n" + entry + format.ProjectCommentsEndLine + "\n"
	task := "\n## Task: Strict\n" +
		"<!-- id: task-1, short_id: 1, state: backlog, priority: 0, " +
		"created: 2026-08-01T12:00:00Z, updated: 2026-08-01T12:00:00Z, " +
		"state_changed: 2026-08-01T12:00:00Z -->\n"

	for name, content := range map[string]string{
		"duplicate block":    header + block + block + task,
		"block after task":   header + task + block,
		"block before name":  schemaMarkerV17 + "\n" + block + "# Project: Comments\n" + task,
		"unterminated block": header + format.ProjectCommentsStartLine + "\n" + entry + task,
		"close without open": header + format.ProjectCommentsEndLine + "\n" + task,
		"empty block": header + format.ProjectCommentsStartLine + "\n" +
			format.ProjectCommentsEndLine + "\n" + task,
		"loose prose first": header + format.ProjectCommentsStartLine + "\nnote\n" +
			format.ProjectCommentsEndLine + "\n" + task,
		"invalid entry timestamp": header + format.ProjectCommentsStartLine + "\n" +
			"<!-- project-comment: " + v17TestCommentIDA + " yesterday -->\nnote\n" +
			format.ProjectCommentsEndLine + "\n" + task,
		"duplicate entry id": header + format.ProjectCommentsStartLine + "\n" + entry + entry +
			format.ProjectCommentsEndLine + "\n" + task,
		"blank entry text": header + format.ProjectCommentsStartLine + "\n" +
			"<!-- project-comment: " + v17TestCommentIDA + " 2026-08-01T12:00:00Z -->\n" +
			format.ProjectCommentsEndLine + "\n" + task,
	} {
		if _, err := ParseV17ProjectDocument(content); err == nil {
			t.Fatalf("%s: reader accepted a document the writer cannot produce:\n%s", name, content)
		}
	}
}

// TestV16ProjectDocumentStaysBlindToProjectComments freezes the historical
// codec. A schema-16 reader learning this block would silently accept documents
// its own writer could never have produced.
func TestV16ProjectDocumentStaysBlindToProjectComments(t *testing.T) {
	document := v17ProjectTestDocument(t)
	document.Comments = []format.ProjectCommentRecord{v17TestComment(v17TestCommentIDA, "note", 0)}
	published, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	downgraded := strings.Replace(published, schemaMarkerV17, schemaMarkerV16, 1)
	if _, err := ParseV16ProjectDocument(downgraded); err == nil {
		t.Fatalf("schema-16 reader accepted a project-comment block:\n%s", downgraded)
	}
}
