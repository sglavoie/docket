package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage/format"
)

// This file is the interior contract of the PROJECT-COMMENTS block. The
// delimiters and the first entry header were already strict; what was not is
// everything after an entry has legitimately opened, where a near-miss header, a
// nested opener, or a malformed closing delimiter used to be collected as prose.
// That absorption is not a cosmetic tolerance: the next canonical rewrite
// escapes the marker, so the apparent entry or block is silently reclassified
// into the preceding comment and stays that way.
//
// Every case below is built from a document the canonical writer produced, with
// exactly one line injected into the block, so a rejection can only be about the
// injected line. Both readers are exercised: the schema codec and the live path
// a real tasks.md takes.

const v17TestCommentIDC = "cccccccc-dddd-4eee-8fff-000000000000"

// interiorGrammarComment is the one legitimately stored entry every negative
// case opens with. The injected line always lands after this entry's body, which
// is what puts it inside the permissive interior rather than in the block's
// already-strict first position.
func interiorGrammarComment() format.ProjectCommentRecord {
	return v17TestComment(v17TestCommentIDA, "the stored note", 0)
}

// interiorGrammarDocument renders canonical schema-17 Markdown carrying the
// given comments and one real task, so every byte outside the block is one the
// writer wrote.
func interiorGrammarDocument(t *testing.T, comments []format.ProjectCommentRecord) string {
	t.Helper()
	document := v17ProjectTestDocument(t)
	document.Comments = comments
	published, err := WriteV17ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV17ProjectDocument: %v", err)
	}
	return published
}

// injectIntoCommentBlock places one raw line directly above the closing
// delimiter of a canonical document's block.
func injectIntoCommentBlock(t *testing.T, published, line string) string {
	t.Helper()
	closing := format.ProjectCommentsEndLine + "\n"
	if !strings.Contains(published, closing) {
		t.Fatalf("canonical document has no comment block to inject into:\n%s", published)
	}
	return strings.Replace(published, closing, line+"\n"+closing, 1)
}

// assertInteriorGrammarRefused holds both readers to the same verdict and to the
// same absence of a partial result: a document refused for its comment grammar
// must not hand back the tasks or the comments it did manage to read.
func assertInteriorGrammarRefused(t *testing.T, name, content string) {
	t.Helper()
	schemaDocument, err := ParseV17ProjectDocument(content)
	if err == nil {
		t.Fatalf("%s: ParseV17ProjectDocument accepted a document the writer cannot produce:\n%s", name, content)
	}
	if schemaDocument.ProjectName != "" || len(schemaDocument.Comments) != 0 || len(schemaDocument.Tasks) != 0 {
		t.Fatalf("%s: refused schema parse still returned a usable document: %+v", name, schemaDocument)
	}
	liveDocument, err := ParseProjectDocument(content)
	if err == nil {
		t.Fatalf("%s: ParseProjectDocument accepted a document the writer cannot produce:\n%s", name, content)
	}
	if liveDocument != nil {
		t.Fatalf("%s: refused live parse still returned a usable document: %+v", name, liveDocument)
	}
}

// assertRecordsMatch compares parsed schema records against what was written,
// value by value, because the whole point of the control is that nothing about a
// legitimate entry moved.
func assertRecordsMatch(t *testing.T, where string, got, want []format.ProjectCommentRecord) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s parsed %d comments, want %d", where, len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Text != want[i].Text || !got[i].CreatedAt.Equal(want[i].CreatedAt) {
			t.Fatalf("%s comment %d = %+v, want %+v", where, i, got[i], want[i])
		}
	}
}

// TestProjectCommentInteriorGrammarIsStrict is the repair contract: raw block
// grammar anywhere inside the block is a malformed document, and legitimate
// prose spelling that same grammar still round-trips untouched.
func TestProjectCommentInteriorGrammarIsStrict(t *testing.T) {
	valid := interiorGrammarDocument(t, []format.ProjectCommentRecord{interiorGrammarComment()})
	if _, err := ParseV17ProjectDocument(valid); err != nil {
		t.Fatalf("the canonical fixture is not readable: %v\n%s", err, valid)
	}

	// A second header that misses the grammar by a timestamp or by trailing junk
	// is not a comment and not prose either: the writer escapes prose that spells
	// a marker, so this shape can only come from something else.
	t.Run("malformed_second_header", func(t *testing.T) {
		stamp := "2026-08-01T13:00:00Z"
		for name, header := range map[string]string{
			"missing_timestamp": "<!-- project-comment: " + v17TestCommentIDB + " -->",
			"trailing_junk":     "<!-- project-comment: " + v17TestCommentIDB + " " + stamp + " --> extra",
			"untrimmed_prefix":  " <!-- project-comment: " + v17TestCommentIDB + " " + stamp + " -->",
		} {
			assertInteriorGrammarRefused(t, name, injectIntoCommentBlock(t, valid, header))
		}
	})

	// An exact opener inside an open block reaches the interior before the
	// top-level duplicate-block guard can ever see it, so the interior is where it
	// has to be refused.
	t.Run("nested_block_opener", func(t *testing.T) {
		assertInteriorGrammarRefused(t, "nested opener",
			injectIntoCommentBlock(t, valid, format.ProjectCommentsStartLine))
	})

	// A near-miss closing delimiter followed by the real one used to close
	// cleanly, with the near miss absorbed into the last comment's body.
	t.Run("malformed_inner_closer", func(t *testing.T) {
		for name, closer := range map[string]string{
			"extra_space":   "<!-- /PROJECT-COMMENTS  -->",
			"no_space":      "<!--/PROJECT-COMMENTS-->",
			"trailing_junk": format.ProjectCommentsEndLine + " extra",
		} {
			assertInteriorGrammarRefused(t, name, injectIntoCommentBlock(t, valid, closer))
		}
	})

	// The control: every marker above, written as ordinary comment text through
	// the canonical writer, survives both readers exactly — including the
	// neighbours it sits between, which is what proves nothing was absorbed.
	t.Run("escaped_literals", func(t *testing.T) {
		hostile := strings.Join([]string{
			format.ProjectCommentsStartLine,
			format.ProjectCommentsEndLine,
			"<!-- /PROJECT-COMMENTS  -->",
			"<!-- project-comment: " + v17TestCommentIDB + " 2026-08-01T13:00:00Z -->",
			"<!-- project-comment: " + v17TestCommentIDB + " -->",
			"a --> b & c <!-- d",
		}, "\n")
		written := []format.ProjectCommentRecord{
			v17TestComment(v17TestCommentIDA, "the earlier note", 0),
			v17TestComment(v17TestCommentIDB, hostile, time.Hour),
			v17TestComment(v17TestCommentIDC, "the later note", 2*time.Hour),
		}
		published := interiorGrammarDocument(t, written)
		if strings.Contains(published, "\n"+format.ProjectCommentsStartLine+"\n"+format.ProjectCommentsStartLine) {
			t.Fatalf("the writer emitted a raw nested opener:\n%s", published)
		}

		parsed, err := ParseV17ProjectDocument(published)
		if err != nil {
			t.Fatalf("ParseV17ProjectDocument: %v\n%s", err, published)
		}
		assertRecordsMatch(t, "the schema reader", parsed.Comments, written)
		if len(parsed.Tasks) != 1 {
			t.Fatalf("the escaped markers swallowed the task list: %+v", parsed.Tasks)
		}

		live, err := ParseProjectDocument(published)
		if err != nil {
			t.Fatalf("ParseProjectDocument: %v\n%s", err, published)
		}
		if len(live.Comments) != len(written) {
			t.Fatalf("the live reader parsed %d comments, want %d", len(live.Comments), len(written))
		}
		for i, want := range written {
			got := live.Comments[i]
			if got.ID != want.ID || got.Text != want.Text || !got.CreatedAt.Equal(want.CreatedAt) {
				t.Fatalf("the live reader comment %d = %+v, want %+v", i, got, want)
			}
		}
		if len(live.Tasks) != 1 {
			t.Fatalf("the escaped markers swallowed the live task list: %+v", live.Tasks)
		}
	})
}
