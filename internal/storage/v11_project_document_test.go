package storage

import (
	"strings"
	"testing"
)

func v11TestDocument() V11ProjectDocument {
	document := V11ProjectDocumentFromV10(v10TestDocument())
	document.Tasks[0].Context = "platform-api"
	return document
}

func TestV11ProjectDocumentRoundTrip(t *testing.T) {
	document := v11TestDocument()
	content, err := WriteV11ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV11ProjectDocument: %v", err)
	}
	if !strings.Contains(content, schemaMarkerV11) || !strings.Contains(content, "context: platform-api") {
		t.Fatalf("document is missing schema-11 data:\n%s", content)
	}
	parsed, err := ParseV11ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV11ProjectDocument: %v", err)
	}
	if parsed.ProjectName != document.ProjectName || len(parsed.Tasks) != len(document.Tasks) {
		t.Fatalf("round trip = %+v", parsed)
	}
	if parsed.Tasks[0].Context != "platform-api" {
		t.Fatalf("context = %q, want platform-api", parsed.Tasks[0].Context)
	}
}

func TestV11ProjectDocumentRejectsSchema10Content(t *testing.T) {
	legacy, err := WriteV10ProjectDocument(v10TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV11ProjectDocument(legacy); err == nil ||
		!strings.Contains(err.Error(), "unsupported v11 task schema 10") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV10ProjectDocumentRejectsSchema11Content(t *testing.T) {
	current, err := WriteV11ProjectDocument(v11TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV10ProjectDocument(current); err == nil ||
		!strings.Contains(err.Error(), "unsupported v10 task schema 11") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV11ProjectDocumentFromV10RebuildsTaskSlice(t *testing.T) {
	source := v10TestDocument()
	lifted := V11ProjectDocumentFromV10(source)
	if len(lifted.Tasks) != len(source.Tasks) {
		t.Fatalf("lifted tasks = %d, want %d", len(lifted.Tasks), len(source.Tasks))
	}
	if lifted.Tasks[0].Context != "" {
		t.Fatalf("lift invented context %q", lifted.Tasks[0].Context)
	}
	lifted.Tasks[0].Context = "changed"
	lifted.Tasks[0].Tags = append(lifted.Tasks[0].Tags, "new-tag")
	if len(source.Tasks[0].Tags) != 0 {
		t.Fatalf("lifted task shared tag slice with source: %v", source.Tasks[0].Tags)
	}
}
