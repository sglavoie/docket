package storage

import (
	"os"
	"strings"
	"testing"
)

func v12TestDocument() V12ProjectDocument {
	document := V12ProjectDocumentFromV11(v11TestDocument())
	document.Tasks[0].DeferCount = 3
	return document
}

func TestV12ProjectDocumentFixtureRoundTripsByteExactly(t *testing.T) {
	fixture, err := os.ReadFile("testdata/v12_project_document.tasks.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	document, err := ParseV12ProjectDocument(string(fixture))
	if err != nil {
		t.Fatalf("ParseV12ProjectDocument: %v", err)
	}
	if len(document.Tasks) != 1 || document.Tasks[0].DeferCount != 3 {
		t.Fatalf("parsed fixture = %+v", document)
	}
	content, err := WriteV12ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV12ProjectDocument: %v", err)
	}
	if content != string(fixture) {
		t.Fatalf("schema-12 fixture changed on round trip\nwant:\n%s\ngot:\n%s", fixture, content)
	}
}

func TestV12ProjectDocumentRejectsSchema11Content(t *testing.T) {
	legacy, err := WriteV11ProjectDocument(v11TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV12ProjectDocument(legacy); err == nil ||
		!strings.Contains(err.Error(), "unsupported v12 task schema 11") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV11ProjectDocumentRejectsSchema12Content(t *testing.T) {
	current, err := WriteV12ProjectDocument(v12TestDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV11ProjectDocument(current); err == nil ||
		!strings.Contains(err.Error(), "unsupported v11 task schema 12") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV12ProjectDocumentFromV11RebuildsTaskSlice(t *testing.T) {
	source := v11TestDocument()
	lifted := V12ProjectDocumentFromV11(source)
	if len(lifted.Tasks) != len(source.Tasks) {
		t.Fatalf("lifted tasks = %d, want %d", len(lifted.Tasks), len(source.Tasks))
	}
	if lifted.Tasks[0].DeferCount != 0 {
		t.Fatalf("lift invented defer count %d", lifted.Tasks[0].DeferCount)
	}
	lifted.Tasks[0].DeferCount = 2
	lifted.Tasks[0].Tags = append(lifted.Tasks[0].Tags, "new-tag")
	if len(source.Tasks[0].Tags) != 0 {
		t.Fatalf("lifted task shared tag slice with source: %v", source.Tasks[0].Tags)
	}
}
