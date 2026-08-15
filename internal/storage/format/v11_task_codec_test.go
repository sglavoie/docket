package format

import (
	"strings"
	"testing"
)

func v11TestRecord() V11TaskRecord {
	record := V11TaskRecord{V10TaskRecord: v10TestRecord(), Context: "platform-api"}
	record.Checklist = []V6ChecklistItem{{Text: "ship", Done: true}}
	return record
}

func TestV11TaskMetadataRoundTripsContextLast(t *testing.T) {
	record := v11TestRecord()
	metadata, err := WriteV11TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV11TaskMetadata: %v", err)
	}
	if !strings.HasSuffix(metadata, "context: platform-api -->") {
		t.Fatalf("context is not appended last: %s", metadata)
	}
	if checklistAt, contextAt := strings.Index(metadata, "checklist:"), strings.Index(metadata, "context:"); checklistAt < 0 || checklistAt >= contextAt {
		t.Fatalf("context is not after checklist: %s", metadata)
	}
	parsed, err := ParseV11TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV11TaskMetadata: %v", err)
	}
	if parsed.Context != record.Context {
		t.Fatalf("context = %q, want %q", parsed.Context, record.Context)
	}
}

func TestV11TaskBodyRoundTrip(t *testing.T) {
	record := v11TestRecord()
	body, err := WriteV11TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV11TaskBody: %v", err)
	}
	parsed, err := ParseV11TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV11TaskBody: %v", err)
	}
	if parsed.Context != record.Context || parsed.Description != record.Description || len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
}

func TestV11TaskContextIsOptional(t *testing.T) {
	record := V11TaskRecord{V10TaskRecord: v10TestRecord()}
	metadata, err := WriteV11TaskMetadata(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(metadata, "context:") {
		t.Fatalf("unset context was written: %s", metadata)
	}
	parsed, err := ParseV11TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV11TaskMetadata: %v", err)
	}
	if parsed.Context != "" {
		t.Fatalf("parsed context = %q, want unset", parsed.Context)
	}
}

func TestV11TaskContextValidationUsesContextVocabulary(t *testing.T) {
	record := v11TestRecord()
	for _, invalid := range []string{"Platform API", "platform:api", "platform,api", "platform;api", "platform\napi"} {
		record.Context = invalid
		_, err := WriteV11TaskMetadata(record)
		if err == nil {
			t.Fatalf("context %q was accepted", invalid)
		}
		if strings.Contains(err.Error(), "tag") || !strings.Contains(err.Error(), "context") {
			t.Fatalf("context %q error = %q, want context vocabulary", invalid, err)
		}
	}
}

func TestV11DoesNotChangeV10Metadata(t *testing.T) {
	record := v10TestRecord()
	before, err := WriteV10TaskMetadata(record)
	if err != nil {
		t.Fatal(err)
	}
	withoutContext, err := WriteV11TaskMetadata(V11TaskRecord{V10TaskRecord: record})
	if err != nil {
		t.Fatal(err)
	}
	if withoutContext != before {
		t.Fatalf("schema-11 empty-context encoding changed inherited bytes\nv10: %s\nv11: %s", before, withoutContext)
	}
}
