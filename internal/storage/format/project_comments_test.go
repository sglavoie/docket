package format

import (
	"strings"
	"testing"
	"time"
)

const (
	blockCommentIDA = "11111111-2222-4333-8444-555555555555"
	blockCommentIDB = "66666666-7777-4888-8999-aaaaaaaaaaaa"
)

func blockComment(id, text string, offset time.Duration) ProjectCommentRecord {
	return ProjectCommentRecord{
		ID:        id,
		Text:      text,
		CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC).Add(offset),
	}
}

// blockBody strips the two delimiters the document frame consumes, which is what
// ParseProjectCommentsBlock is handed.
func blockBody(t *testing.T, rendered string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) < 2 || lines[0] != ProjectCommentsStartLine ||
		lines[len(lines)-1] != ProjectCommentsEndLine {
		t.Fatalf("rendered block is not delimited:\n%s", rendered)
	}
	return lines[1 : len(lines)-1]
}

// TestProjectCommentsBlockRoundTripsMultilineProse is the codec's core claim:
// what the writer emits is what the reader returns, bodies included.
func TestProjectCommentsBlockRoundTripsMultilineProse(t *testing.T) {
	records := []ProjectCommentRecord{
		blockComment(blockCommentIDA, "first line\n\nthird line", 0),
		blockComment(blockCommentIDB, "single line", time.Hour),
	}
	rendered, err := WriteProjectCommentsBlock(records)
	if err != nil {
		t.Fatalf("WriteProjectCommentsBlock: %v", err)
	}
	parsed, err := ParseProjectCommentsBlock(blockBody(t, rendered))
	if err != nil {
		t.Fatalf("ParseProjectCommentsBlock: %v", err)
	}
	if len(parsed) != len(records) {
		t.Fatalf("parsed %d records, want %d", len(parsed), len(records))
	}
	for i, want := range records {
		if parsed[i].ID != want.ID || parsed[i].Text != want.Text || !parsed[i].CreatedAt.Equal(want.CreatedAt) {
			t.Fatalf("record %d = %+v, want %+v", i, parsed[i], want)
		}
	}
}

// TestProjectCommentsBlockEscapesItsOwnGrammar is why the block reuses the prose
// escaping: a body spelling a delimiter must come back as text rather than
// closing the block or opening an entry.
func TestProjectCommentsBlockEscapesItsOwnGrammar(t *testing.T) {
	hostile := strings.Join([]string{
		ProjectCommentsEndLine,
		"<!-- project-comment: " + blockCommentIDB + " 2026-08-01T12:00:00Z -->",
		"a --> b & c",
	}, "\n")
	rendered, err := WriteProjectCommentsBlock([]ProjectCommentRecord{blockComment(blockCommentIDA, hostile, 0)})
	if err != nil {
		t.Fatalf("WriteProjectCommentsBlock: %v", err)
	}
	parsed, err := ParseProjectCommentsBlock(blockBody(t, rendered))
	if err != nil {
		t.Fatalf("ParseProjectCommentsBlock: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Text != hostile {
		t.Fatalf("hostile prose changed: %+v", parsed)
	}
}

// TestWriteProjectCommentsBlockIsCanonical pins the two properties a revision
// hash depends on: an empty collection writes nothing at all, and the order and
// zone come only from stored values.
func TestWriteProjectCommentsBlockIsCanonical(t *testing.T) {
	if rendered, err := WriteProjectCommentsBlock(nil); err != nil || rendered != "" {
		t.Fatalf("empty collection wrote %q (err %v), want no block", rendered, err)
	}
	zone := time.FixedZone("test-plus-two", 2*60*60)
	unordered := []ProjectCommentRecord{
		blockComment(blockCommentIDB, "later", time.Hour),
		{ID: blockCommentIDA, Text: "zoned", CreatedAt: time.Date(2026, 8, 1, 14, 0, 0, 0, zone)},
	}
	rendered, err := WriteProjectCommentsBlock(unordered)
	if err != nil {
		t.Fatalf("WriteProjectCommentsBlock: %v", err)
	}
	if strings.Index(rendered, blockCommentIDA) > strings.Index(rendered, blockCommentIDB) {
		t.Fatalf("records were written out of canonical order:\n%s", rendered)
	}
	if !strings.Contains(rendered, "2026-08-01T12:00:00Z") {
		t.Fatalf("stored instant was not normalized to UTC:\n%s", rendered)
	}
	// Sorting must not reorder the caller's slice, which the document layer still
	// holds.
	if unordered[0].ID != blockCommentIDB {
		t.Fatal("writing the block reordered the caller's records")
	}
}

// TestWriteProjectCommentsBlockRefusesUnstorableRecords keeps the writer as
// strict as the reader, so nothing a loaded project would be refused for can
// reach bytes.
func TestWriteProjectCommentsBlockRefusesUnstorableRecords(t *testing.T) {
	for name, records := range map[string][]ProjectCommentRecord{
		"duplicate id": {
			blockComment(blockCommentIDA, "one", 0),
			blockComment(blockCommentIDA, "two", time.Hour),
		},
		"invalid id":   {blockComment("comment-1", "one", 0)},
		"blank text":   {blockComment(blockCommentIDA, "  \n ", 0)},
		"zero instant": {{ID: blockCommentIDA, Text: "one"}},
	} {
		if _, err := WriteProjectCommentsBlock(records); err == nil {
			t.Fatalf("%s: writer accepted a collection it cannot represent", name)
		}
	}
}

// TestParseProjectCommentsBlockRefusesShapesTheWriterCannotProduce is the other
// half of that contract: the reader repairs nothing.
func TestParseProjectCommentsBlockRefusesShapesTheWriterCannotProduce(t *testing.T) {
	header := "<!-- project-comment: " + blockCommentIDA + " 2026-08-01T12:00:00Z -->"
	for name, lines := range map[string][]string{
		"prose before the first header": {"loose note", header, "note"},
		"unreadable timestamp":          {"<!-- project-comment: " + blockCommentIDA + " yesterday -->", "note"},
		"trailing header text":          {header + " extra", "note"},
		"blank body":                    {header},
		"duplicate id":                  {header, "one", header, "two"},
		// An empty collection writes no delimiters at all, so a bare pair is not a
		// document this codec produced.
		"empty block": nil,
	} {
		if _, err := ParseProjectCommentsBlock(lines); err == nil {
			t.Fatalf("%s: reader accepted %q", name, lines)
		}
	}
}
