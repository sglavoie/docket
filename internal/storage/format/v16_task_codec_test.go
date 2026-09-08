package format

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func v16TestRecord() V16TaskRecord {
	return V16TaskRecord{V15TaskRecord: v15TestRecord(), ResumeNote: "waiting on the staging deploy"}
}

func TestV16TaskBodyRoundTripsTheResumeNote(t *testing.T) {
	record := v16TestRecord()
	body, err := WriteV16TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV16TaskBody: %v", err)
	}
	if !strings.Contains(body, "<!-- RESUME -->\nwaiting on the staging deploy\n<!-- /RESUME -->\n") {
		t.Fatalf("body does not carry a canonical RESUME block:\n%s", body)
	}
	parsed, err := ParseV16TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV16TaskBody: %v", err)
	}
	if parsed.ResumeNote != record.ResumeNote {
		t.Fatalf("resume note = %q, want %q", parsed.ResumeNote, record.ResumeNote)
	}
	if !parsed.Review || !reflect.DeepEqual(parsed.Dependencies, record.Dependencies) ||
		parsed.Description != record.Description || len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost inherited content: %+v", parsed)
	}
	rewritten, err := WriteV16TaskBody(parsed)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if rewritten != body {
		t.Fatalf("encoding is not deterministic\nfirst:\n%s\nsecond:\n%s", body, rewritten)
	}
}

// TestV16ResumeNoteBlockLeadsTheBody pins the canonical order. The reader does
// not depend on it, but the writer must produce one arrangement or every stored
// revision would move with the mood of whoever saved last.
func TestV16ResumeNoteBlockLeadsTheBody(t *testing.T) {
	body, err := WriteV16TaskBody(v16TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	resume := strings.Index(body, "<!-- RESUME -->")
	description := strings.Index(body, "<!-- DESCRIPTION -->")
	comments := strings.Index(body, "<!-- COMMENTS -->")
	if resume < 0 || description < 0 || comments < 0 || resume >= description || description >= comments {
		t.Fatalf("block order = resume:%d description:%d comments:%d\n%s", resume, description, comments, body)
	}
}

// TestV16AbsentResumeNoteMatchesV15Bytes is the byte-equality guarantee the
// schema-18 migration rests on: a task with no note must serialize exactly as its
// schema-15 predecessor did.
func TestV16AbsentResumeNoteMatchesV15Bytes(t *testing.T) {
	base := v15TestRecord()
	v15Body, err := WriteV15TaskBody(base)
	if err != nil {
		t.Fatal(err)
	}
	v16Body, err := WriteV16TaskBody(V16TaskRecord{V15TaskRecord: base})
	if err != nil {
		t.Fatal(err)
	}
	if v16Body != v15Body {
		t.Fatalf("an unnoted task changed inherited bytes\nv15:\n%s\nv16:\n%s", v15Body, v16Body)
	}
	if strings.Contains(v16Body, "RESUME") {
		t.Fatalf("an empty note was not canonically omitted:\n%s", v16Body)
	}
	parsed, err := ParseV16TaskBody(base.Title, v16Body)
	if err != nil {
		t.Fatalf("ParseV16TaskBody: %v", err)
	}
	if parsed.ResumeNote != "" {
		t.Fatalf("an omitted block parsed as note %q", parsed.ResumeNote)
	}
}

// TestV15RejectsAResumeBlock keeps schema 15 strict and migration-only: a frozen
// reader that learned a newer block would stop being a fixed target for the
// archives a migration validates against.
func TestV15RejectsAResumeBlock(t *testing.T) {
	body, err := WriteV16TaskBody(v16TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV15TaskBody("Resume", body); err == nil {
		t.Fatalf("schema 15 accepted a RESUME block:\n%s", body)
	}
}

// TestV16ResumeNoteEscapesBlockDelimiters keeps prose out of the block grammar:
// a note that spells a delimiter must survive verbatim and must not be readable
// back as structure.
func TestV16ResumeNoteEscapesBlockDelimiters(t *testing.T) {
	for _, note := range []string{
		"<!-- /RESUME -->",
		"<!-- DESCRIPTION -->",
		"## Task: not a heading",
		`\<!-- RESUME --> already escaped`,
		"résumé ✅ 続きから — unicode survives",
	} {
		t.Run(note, func(t *testing.T) {
			record := v16TestRecord()
			record.ResumeNote = note
			body, err := WriteV16TaskBody(record)
			if err != nil {
				t.Fatalf("WriteV16TaskBody: %v", err)
			}
			parsed, err := ParseV16TaskBody(record.Title, body)
			if err != nil {
				t.Fatalf("ParseV16TaskBody: %v", err)
			}
			if parsed.ResumeNote != note {
				t.Fatalf("note changed\n got: %q\nwant: %q", parsed.ResumeNote, note)
			}
		})
	}
}

// TestV16ResumeNoteLengthBoundary pins the bound in characters rather than
// bytes, which is what makes the allowance the same in every script.
func TestV16ResumeNoteLengthBoundary(t *testing.T) {
	record := v16TestRecord()
	record.ResumeNote = strings.Repeat("é", models.MaxResumeNoteLength)
	body, err := WriteV16TaskBody(record)
	if err != nil {
		t.Fatalf("a note of exactly %d characters was refused: %v", models.MaxResumeNoteLength, err)
	}
	parsed, err := ParseV16TaskBody(record.Title, body)
	if err != nil || parsed.ResumeNote != record.ResumeNote {
		t.Fatalf("boundary note did not round trip: %v", err)
	}
	record.ResumeNote = strings.Repeat("é", models.MaxResumeNoteLength+1)
	if _, err := WriteV16TaskBody(record); err == nil {
		t.Fatalf("a note of %d characters was accepted", models.MaxResumeNoteLength+1)
	}
}

// TestV16ResumeNoteWriterRefusesShapesTheReaderCannotRead keeps the two halves
// in agreement: the writer must never produce a block the reader refuses.
func TestV16ResumeNoteWriterRefusesShapesTheReaderCannotRead(t *testing.T) {
	for name, note := range map[string]string{
		"line break":         "one\ntwo",
		"carriage return":    "one\rtwo",
		"leading whitespace": "  padded",
		"trailing space":     "padded ",
	} {
		t.Run(name, func(t *testing.T) {
			record := v16TestRecord()
			record.ResumeNote = note
			if _, err := WriteV16TaskBody(record); err == nil {
				t.Fatalf("the writer accepted an unrepresentable note %q", note)
			}
		})
	}
}

// TestV16MalformedResumeBlocksAreRefused covers the shapes the canonical writer
// can never produce, so accepting one would mean guessing at a file something
// else wrote.
func TestV16MalformedResumeBlocksAreRefused(t *testing.T) {
	metadata, err := WriteV15TaskMetadata(v15TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	for name, block := range map[string]string{
		"duplicate":    "<!-- RESUME -->\none\n<!-- /RESUME -->\n<!-- RESUME -->\ntwo\n<!-- /RESUME -->\n",
		"unterminated": "<!-- RESUME -->\nlater\n",
		"multi-line":   "<!-- RESUME -->\none\ntwo\n<!-- /RESUME -->\n",
		"empty":        "<!-- RESUME -->\n<!-- /RESUME -->\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseV16TaskBody("Resume", metadata+"\n"+block); err == nil {
				t.Fatalf("the reader accepted a malformed RESUME block:\n%s", block)
			}
		})
	}
}
