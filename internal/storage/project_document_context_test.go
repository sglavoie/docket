package storage

import (
	"strings"
	"testing"
)

func TestLiveProjectDocumentRoundTripsTaskContext(t *testing.T) {
	project := mergeTestDoc("Context", mergeTestTask("task-1", "Contextual"))
	project.Tasks[0].Context = "platform-api"

	content, err := project.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	if !strings.Contains(content, schemaMarkerV11) || !strings.Contains(content, "context: platform-api") {
		t.Fatalf("live document omitted schema-11 context:\n%s", content)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if parsed.SchemaVersion != SchemaVersionV11 || len(parsed.Tasks) != 1 || parsed.Tasks[0].Context != "platform-api" {
		t.Fatalf("parsed live document = %+v", parsed)
	}
}

func TestLiveProjectDocumentRejectsInvalidTaskContext(t *testing.T) {
	project := mergeTestDoc("Context", mergeTestTask("task-1", "Contextual"))
	project.Tasks[0].Context = "Platform API"

	_, err := project.WriteCurrent()
	if err == nil || strings.Contains(err.Error(), "tag") || !strings.Contains(err.Error(), "context") {
		t.Fatalf("WriteCurrent error = %v, want context validation error", err)
	}
}
