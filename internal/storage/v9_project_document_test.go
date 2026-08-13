package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage/format"
)

func v9TestDocument() V9ProjectDocument {
	document := v8TestDocument()
	document.Tasks[0].SnoozedUntil = "2026-05-14"
	return document
}

func TestV9ProjectDocumentRoundTrip(t *testing.T) {
	document := v9TestDocument()
	content, err := WriteV9ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV9ProjectDocument: %v", err)
	}
	if !strings.Contains(content, schemaMarkerV9) {
		t.Fatalf("document is missing the schema-9 marker:\n%s", content)
	}
	if !strings.Contains(content, "deferred: 2026-05-14") || strings.Contains(content, "snoozed:") {
		t.Fatalf("document does not carry the renamed key:\n%s", content)
	}
	parsed, err := ParseV9ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV9ProjectDocument: %v", err)
	}
	if parsed.ProjectName != document.ProjectName || len(parsed.Tasks) != len(document.Tasks) {
		t.Fatalf("round trip = %+v", parsed)
	}
	if parsed.Tasks[0].SnoozedUntil != document.Tasks[0].SnoozedUntil {
		t.Fatalf("deferral date = %q, want %q", parsed.Tasks[0].SnoozedUntil, document.Tasks[0].SnoozedUntil)
	}
}

func TestV9ProjectDocumentRejectsSchema8Content(t *testing.T) {
	legacy, err := WriteV8ProjectDocument(v9TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV9ProjectDocument(legacy); err == nil ||
		!strings.Contains(err.Error(), "unsupported v9 task schema 8") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV8ProjectDocumentRejectsSchema9Content(t *testing.T) {
	current, err := WriteV9ProjectDocument(v9TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV8ProjectDocument(current); err == nil ||
		!strings.Contains(err.Error(), "unsupported v8 task schema 9") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

// TestV9ProjectDocumentFromV8RenamesOnlyTheMetadataKey compares whole documents
// byte for byte, so any other difference the lift introduces fails here rather
// than surviving into the migration that will call it.
func TestV9ProjectDocumentFromV8RenamesOnlyTheMetadataKey(t *testing.T) {
	source := v9TestDocument()
	legacy, err := WriteV8ProjectDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	lifted, err := WriteV9ProjectDocument(V9ProjectDocumentFromV8(source))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.NewReplacer(
		schemaMarkerV8, schemaMarkerV9,
		", snoozed: ", ", deferred: ",
	).Replace(legacy)
	if lifted != want {
		t.Fatalf("lifted document =\n%s\nwant\n%s", lifted, want)
	}
}

// TestV9ProjectDocumentKeepsNarrativeDeferralWords guards the rename against a
// document-level text replace: descriptions and comments can contain either word.
func TestV9ProjectDocumentKeepsNarrativeDeferralWords(t *testing.T) {
	document := v9TestDocument()
	document.Tasks[0].Description = "was snoozed: until the deferred review"
	document.Tasks[0].Comments = []format.V6TaskComment{
		{Text: "snoozed: see the deferred plan", CreatedAt: document.Tasks[0].UpdatedAt},
	}
	content, err := WriteV9ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV9ProjectDocument: %v", err)
	}
	parsed, err := ParseV9ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV9ProjectDocument: %v", err)
	}
	if parsed.Tasks[0].Description != document.Tasks[0].Description {
		t.Fatalf("description = %q, want %q", parsed.Tasks[0].Description, document.Tasks[0].Description)
	}
	if len(parsed.Tasks[0].Comments) != 1 ||
		parsed.Tasks[0].Comments[0].Text != document.Tasks[0].Comments[0].Text {
		t.Fatalf("comments = %+v", parsed.Tasks[0].Comments)
	}
}
