package storage

import (
	"strings"
	"testing"
)

func TestLiveProjectDocumentRoundTripsSchema12Fields(t *testing.T) {
	project := mergeTestDoc("Context", mergeTestTask("task-1", "Contextual"))
	project.Tasks[0].Context = "platform-api"
	project.Tasks[0].DeferCount = 3

	content, err := project.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	if !strings.Contains(content, schemaMarkerV17) || !strings.Contains(content, "context: platform-api") ||
		!strings.Contains(content, "defer_count: 3") {
		t.Fatalf("live document omitted schema-12 fields:\n%s", content)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if parsed.SchemaVersion != SchemaVersionV17 || len(parsed.Tasks) != 1 ||
		parsed.Tasks[0].Context != "platform-api" || parsed.Tasks[0].DeferCount != 3 {
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
