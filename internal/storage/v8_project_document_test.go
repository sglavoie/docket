package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/storage/format"
)

func v8TestDocument() V8ProjectDocument {
	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 4, 2, 10, 30, 0, 0, time.UTC)
	stateChanged := time.Date(2026, 3, 20, 8, 15, 0, 0, time.UTC)
	return V8ProjectDocument{
		ProjectName: "Sonar",
		Tasks: []format.V8TaskRecord{
			{
				V6TaskRecord: format.V6TaskRecord{
					ID: "task-1", ShortID: 1, Title: "Parent", State: "waiting",
					Priority: 2, CreatedAt: created, UpdatedAt: updated,
				},
				StateChangedAt: stateChanged,
			},
			{
				V6TaskRecord: format.V6TaskRecord{
					ID: "task-2", ShortID: 2, Title: "Child", State: "backlog",
					Priority: 0, ParentID: "task-1", CreatedAt: created, UpdatedAt: updated,
				},
				StateChangedAt: created,
			},
		},
	}
}

func TestV8ProjectDocumentRoundTrip(t *testing.T) {
	document := v8TestDocument()
	content, err := WriteV8ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV8ProjectDocument: %v", err)
	}
	if !strings.Contains(content, "<!-- tt-schema: 8 -->") {
		t.Fatalf("document is missing the schema-8 marker:\n%s", content)
	}
	parsed, err := ParseV8ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV8ProjectDocument: %v", err)
	}
	if parsed.ProjectName != document.ProjectName || len(parsed.Tasks) != 2 {
		t.Fatalf("round trip = %+v", parsed)
	}
	for i, task := range parsed.Tasks {
		if !task.StateChangedAt.Equal(document.Tasks[i].StateChangedAt) {
			t.Fatalf("task %q state clock = %v, want %v", task.ID, task.StateChangedAt, document.Tasks[i].StateChangedAt)
		}
	}
}

func TestV8ProjectDocumentRejectsSchema7Content(t *testing.T) {
	legacy, err := WriteV7ProjectDocument(v7ProjectDocumentFromV8(v8TestDocument()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV8ProjectDocument(legacy); err == nil ||
		!strings.Contains(err.Error(), "unsupported v8 task schema 7") {
		t.Fatalf("parse error = %v, want a schema rejection", err)
	}
}

func TestV8ProjectDocumentFromV7BackfillsEveryTask(t *testing.T) {
	source := v7ProjectDocumentFromV8(v8TestDocument())
	lifted := V8ProjectDocumentFromV7(source)
	if len(lifted.Tasks) != len(source.Tasks) {
		t.Fatalf("lifted tasks = %d, want %d", len(lifted.Tasks), len(source.Tasks))
	}
	for i, task := range lifted.Tasks {
		if !task.StateChangedAt.Equal(source.Tasks[i].UpdatedAt) {
			t.Fatalf("task %q state clock = %v, want %v", task.ID, task.StateChangedAt, source.Tasks[i].UpdatedAt)
		}
	}
}

func TestV8ProjectDocumentRejectsMissingStateClock(t *testing.T) {
	document := v8TestDocument()
	document.Tasks[0].StateChangedAt = time.Time{}
	if err := ValidateV8ProjectDocument(document); err == nil ||
		!strings.Contains(err.Error(), "requires a state changed timestamp") {
		t.Fatalf("validation error = %v, want a missing state clock rejection", err)
	}
}
