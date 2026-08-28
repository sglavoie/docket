package format

import (
	"reflect"
	"strings"
	"testing"
)

func v14TestRecord() V14TaskRecord {
	return V14TaskRecord{
		V13TaskRecord: v13TestRecord(),
		Dependencies: []V14TaskDependency{
			{ProjectID: "project-beta", TaskID: "task-9"},
			{ProjectID: "project-alpha", TaskID: "task-1"},
		},
	}
}

func TestV14TaskMetadataRoundTripsDependenciesInOrderLast(t *testing.T) {
	record := v14TestRecord()
	metadata, err := WriteV14TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV14TaskMetadata: %v", err)
	}
	if !strings.HasSuffix(metadata, "depends: project-beta:task-9;project-alpha:task-1 -->") {
		t.Fatalf("depends is not the canonical trailing field: %s", metadata)
	}
	parsed, err := ParseV14TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV14TaskMetadata: %v", err)
	}
	if !reflect.DeepEqual(parsed.Dependencies, record.Dependencies) {
		t.Fatalf("dependencies = %+v, want %+v", parsed.Dependencies, record.Dependencies)
	}
	// Writing what was parsed must reproduce the same bytes, or a load/save with
	// no user edit would churn the file and invalidate every stored revision.
	rewritten, err := WriteV14TaskMetadata(parsed)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if rewritten != metadata {
		t.Fatalf("encoding is not deterministic\nfirst:  %s\nsecond: %s", metadata, rewritten)
	}
}

func TestV14TaskBodyRoundTrip(t *testing.T) {
	record := v14TestRecord()
	body, err := WriteV14TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV14TaskBody: %v", err)
	}
	parsed, err := ParseV14TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV14TaskBody: %v", err)
	}
	if !reflect.DeepEqual(parsed.Dependencies, record.Dependencies) ||
		parsed.DueTime != record.DueTime || parsed.DeferCount != record.DeferCount ||
		parsed.Context != record.Context || parsed.Description != record.Description ||
		len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
}

// TestV14DependenciesAreOptionalAndMatchV13WhenAbsent is the byte-equality
// guarantee the migration rests on: a task with no dependencies must serialize
// exactly as its schema-13 predecessor did.
func TestV14DependenciesAreOptionalAndMatchV13WhenAbsent(t *testing.T) {
	if !reflect.DeepEqual(v14RequiredMetadataKeys, v13RequiredMetadataKeys) {
		t.Fatalf("required metadata changed: v14=%v v13=%v", v14RequiredMetadataKeys, v13RequiredMetadataKeys)
	}
	base := v13TestRecord()
	v13Metadata, err := WriteV13TaskMetadata(base)
	if err != nil {
		t.Fatal(err)
	}
	v14Metadata, err := WriteV14TaskMetadata(V14TaskRecord{V13TaskRecord: base})
	if err != nil {
		t.Fatal(err)
	}
	if v14Metadata != v13Metadata {
		t.Fatalf("empty dependencies changed inherited bytes\nv13: %s\nv14: %s", v13Metadata, v14Metadata)
	}
	if strings.Contains(v14Metadata, "depends") {
		t.Fatalf("an empty dependency list was not canonically omitted: %s", v14Metadata)
	}
	parsed, err := ParseV14TaskMetadata(v14Metadata)
	if err != nil {
		t.Fatalf("ParseV14TaskMetadata: %v", err)
	}
	if len(parsed.Dependencies) != 0 {
		t.Fatalf("dependencies = %+v, want none", parsed.Dependencies)
	}
}

// TestV13RejectsDependsMetadata keeps schema 13 strict and migration-only: it
// must never learn to read the key schema 14 introduced, or a downgraded reader
// would silently drop a persisted dependency list.
func TestV13RejectsDependsMetadata(t *testing.T) {
	metadata, err := WriteV14TaskMetadata(v14TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseV13TaskMetadata(metadata)
	if err == nil || !strings.Contains(err.Error(), "unknown v13 metadata field \"depends\"") {
		t.Fatalf("ParseV13TaskMetadata error = %v, want an unknown depends field refusal", err)
	}
	if v13MetadataKeys["depends"] {
		t.Fatal("schema 13 admitted the depends key into its metadata key set")
	}
}

func TestV14MalformedDependsMetadataIsRefused(t *testing.T) {
	valid, err := WriteV14TaskMetadata(V14TaskRecord{V13TaskRecord: v13TestRecord()})
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"empty value":       "",
		"missing separator": "project-alpha",
		"empty project id":  ":task-1",
		"empty task id":     "project-alpha:",
		"empty entry":       "project-alpha:task-1;",
		"duplicate entry":   "project-alpha:task-1;project-alpha:task-1",
		"reserved rune":     "project alpha:task-1",
	} {
		comment := strings.TrimSuffix(valid, " -->") + ", depends: " + value + " -->"
		if _, err := ParseV14TaskMetadata(comment); err == nil {
			t.Fatalf("%s: ParseV14TaskMetadata(%q) succeeded, want refusal", name, comment)
		}
	}
}

func TestV14WriterRefusesAnUnpersistableDependency(t *testing.T) {
	record := V14TaskRecord{
		V13TaskRecord: v13TestRecord(),
		Dependencies:  []V14TaskDependency{{ProjectID: "project-alpha", TaskID: ""}},
	}
	if _, err := WriteV14TaskMetadata(record); err == nil {
		t.Fatal("writer accepted a dependency with an empty task id")
	}
	record.Dependencies = []V14TaskDependency{
		{ProjectID: "p", TaskID: "a"},
		{ProjectID: "p", TaskID: "a"},
	}
	if err := ValidateV14TaskRecord(record); err == nil {
		t.Fatal("validator accepted a duplicate dependency")
	}
}
