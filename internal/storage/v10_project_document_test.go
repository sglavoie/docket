package storage

import (
	"strings"
	"testing"
	"time"
)

func v10TestDocument() V10ProjectDocument {
	document := v8TestDocument()
	document.Tasks[0].SnoozedUntil = "2026-05-14T09:00:00Z"
	return document
}

func TestV10ProjectDocumentRoundTrip(t *testing.T) {
	document := v10TestDocument()
	content, err := WriteV10ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV10ProjectDocument: %v", err)
	}
	if !strings.Contains(content, schemaMarkerV10) {
		t.Fatalf("document is missing the schema-10 marker:\n%s", content)
	}
	if !strings.Contains(content, "deferred: 2026-05-14T09:00:00Z") {
		t.Fatalf("document does not carry the deferral instant:\n%s", content)
	}
	parsed, err := ParseV10ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV10ProjectDocument: %v", err)
	}
	if parsed.ProjectName != document.ProjectName || len(parsed.Tasks) != len(document.Tasks) {
		t.Fatalf("round trip = %+v", parsed)
	}
	if parsed.Tasks[0].SnoozedUntil != document.Tasks[0].SnoozedUntil {
		t.Fatalf("deferral instant = %q, want %q", parsed.Tasks[0].SnoozedUntil, document.Tasks[0].SnoozedUntil)
	}
}

func TestV10ProjectDocumentRejectsSchema9Content(t *testing.T) {
	legacy, err := WriteV9ProjectDocument(v9TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV10ProjectDocument(legacy); err == nil ||
		!strings.Contains(err.Error(), "unsupported v10 task schema 9") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV9ProjectDocumentRejectsSchema10Content(t *testing.T) {
	current, err := WriteV10ProjectDocument(v10TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV9ProjectDocument(current); err == nil ||
		!strings.Contains(err.Error(), "unsupported v9 task schema 10") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

// TestV10ProjectDocumentFromV9LiftsToLocalMidnight pins the one behavioral
// equivalence the schema bump rests on: a bare date meant the instant that day
// began in the reader's own location, so the lift has to resolve it there and
// nowhere else.
func TestV10ProjectDocumentFromV9LiftsToLocalMidnight(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	source := v9TestDocument()
	lifted, err := V10ProjectDocumentFromV9(source, location)
	if err != nil {
		t.Fatalf("V10ProjectDocumentFromV9: %v", err)
	}
	if lifted.Tasks[0].SnoozedUntil != "2026-05-14T00:00:00-07:00" {
		t.Fatalf("lifted deferral = %q, want local midnight", lifted.Tasks[0].SnoozedUntil)
	}
	if source.Tasks[0].SnoozedUntil != "2026-05-14" {
		t.Fatalf("lift rewrote the source document: %q", source.Tasks[0].SnoozedUntil)
	}
	if lifted.Tasks[1].SnoozedUntil != "" {
		t.Fatalf("lift invented a deferral for an undeferred task: %q", lifted.Tasks[1].SnoozedUntil)
	}
	if _, err := WriteV10ProjectDocument(lifted); err != nil {
		t.Fatalf("WriteV10ProjectDocument rejected the lifted document: %v", err)
	}
}
