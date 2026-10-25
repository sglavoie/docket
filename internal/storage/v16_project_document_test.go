package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/storage/format"
)

// TestV16ProjectDocumentLiftFromV15AddsNoResumeNote is what makes the schema-18
// migration a marker change: lifting a document must leave every task without a
// note, and a task without a note writes no block.
func TestV16ProjectDocumentLiftFromV15AddsNoResumeNote(t *testing.T) {
	source := V15ProjectDocument{ProjectName: "Resume", Tasks: []format.V15TaskRecord{v15ProjectTestRecord(t)}}
	lifted := V16ProjectDocumentFromV15(source)
	if len(lifted.Tasks) != 1 || lifted.Tasks[0].ResumeNote != "" {
		t.Fatalf("lift = %+v, want one task with no resume note", lifted.Tasks)
	}
	published, err := WriteV16ProjectDocument(lifted)
	if err != nil {
		t.Fatalf("WriteV16ProjectDocument: %v", err)
	}
	if strings.Contains(published, "RESUME") {
		t.Fatalf("lifted document wrote a RESUME block:\n%s", published)
	}
	if !strings.Contains(published, schemaMarkerV16) {
		t.Fatalf("lifted document is not schema 16:\n%s", published)
	}
	original, err := WriteV15ProjectDocument(source)
	if err != nil {
		t.Fatalf("WriteV15ProjectDocument: %v", err)
	}
	if strings.Replace(published, schemaMarkerV16, schemaMarkerV15, 1) != original {
		t.Fatalf("lift changed bytes beyond the marker\nv15: %s\nv16: %s", original, published)
	}
}

// TestV16ProjectDocumentRoundTripsTheResumeNote covers the document level rather
// than the body: the note has to survive the heading scan and the project-wide
// validation, not just its own block.
func TestV16ProjectDocumentRoundTripsTheResumeNote(t *testing.T) {
	document := V16ProjectDocumentFromV15(V15ProjectDocument{
		ProjectName: "Resume", Tasks: []format.V15TaskRecord{v15ProjectTestRecord(t)},
	})
	document.Tasks[0].ResumeNote = "stopped at the failing case"
	published, err := WriteV16ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV16ProjectDocument: %v", err)
	}
	parsed, err := ParseV16ProjectDocument(published)
	if err != nil {
		t.Fatalf("ParseV16ProjectDocument: %v", err)
	}
	if len(parsed.Tasks) != 1 || parsed.Tasks[0].ResumeNote != document.Tasks[0].ResumeNote {
		t.Fatalf("resume note did not round trip: %+v", parsed.Tasks)
	}
}

// TestV16ProjectDocumentRejectsAnUnrepresentableResumeNote keeps the document
// writer as strict as the model gate, so an invalid note never reaches bytes.
func TestV16ProjectDocumentRejectsAnUnrepresentableResumeNote(t *testing.T) {
	document := V16ProjectDocumentFromV15(V15ProjectDocument{
		ProjectName: "Resume", Tasks: []format.V15TaskRecord{v15ProjectTestRecord(t)},
	})
	document.Tasks[0].ResumeNote = "one\ntwo"
	if _, err := WriteV16ProjectDocument(document); err == nil {
		t.Fatal("WriteV16ProjectDocument accepted a multi-line resume note")
	}
}

func v15ProjectTestRecord(t *testing.T) format.V15TaskRecord {
	t.Helper()
	record, err := currentTaskRecordFromModel(mergeTestTask("task-1", "Lifted"))
	if err != nil {
		t.Fatal(err)
	}
	return record.V15TaskRecord
}
