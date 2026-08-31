package format

import (
	"reflect"
	"strings"
	"testing"
)

func v15TestRecord() V15TaskRecord {
	return V15TaskRecord{V14TaskRecord: v14TestRecord(), Review: true}
}

func TestV15TaskMetadataRoundTripsTheReviewFlagLast(t *testing.T) {
	record := v15TestRecord()
	metadata, err := WriteV15TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV15TaskMetadata: %v", err)
	}
	if !strings.HasSuffix(metadata, "review: true -->") {
		t.Fatalf("review is not the canonical trailing field: %s", metadata)
	}
	parsed, err := ParseV15TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV15TaskMetadata: %v", err)
	}
	if !parsed.Review {
		t.Fatalf("review flag was lost: %s", metadata)
	}
	if !reflect.DeepEqual(parsed.Dependencies, record.Dependencies) {
		t.Fatalf("dependencies = %+v, want %+v", parsed.Dependencies, record.Dependencies)
	}
	// Writing what was parsed must reproduce the same bytes, or a load/save with
	// no user edit would churn the file and invalidate every stored revision.
	rewritten, err := WriteV15TaskMetadata(parsed)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if rewritten != metadata {
		t.Fatalf("encoding is not deterministic\nfirst:  %s\nsecond: %s", metadata, rewritten)
	}
}

func TestV15TaskBodyRoundTrip(t *testing.T) {
	record := v15TestRecord()
	body, err := WriteV15TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV15TaskBody: %v", err)
	}
	parsed, err := ParseV15TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV15TaskBody: %v", err)
	}
	if !parsed.Review || !reflect.DeepEqual(parsed.Dependencies, record.Dependencies) ||
		parsed.DueTime != record.DueTime || parsed.DeferCount != record.DeferCount ||
		parsed.Context != record.Context || parsed.Description != record.Description ||
		len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
}

// TestV15ReviewIsOptionalAndMatchesV14WhenOff is the byte-equality guarantee the
// migration rests on: a task that is not up for review must serialize exactly as
// its schema-14 predecessor did.
func TestV15ReviewIsOptionalAndMatchesV14WhenOff(t *testing.T) {
	if !reflect.DeepEqual(v15RequiredMetadataKeys, v14RequiredMetadataKeys) {
		t.Fatalf("required metadata changed: v15=%v v14=%v", v15RequiredMetadataKeys, v14RequiredMetadataKeys)
	}
	base := v14TestRecord()
	v14Metadata, err := WriteV14TaskMetadata(base)
	if err != nil {
		t.Fatal(err)
	}
	v15Metadata, err := WriteV15TaskMetadata(V15TaskRecord{V14TaskRecord: base})
	if err != nil {
		t.Fatal(err)
	}
	if v15Metadata != v14Metadata {
		t.Fatalf("an unflagged task changed inherited bytes\nv14: %s\nv15: %s", v14Metadata, v15Metadata)
	}
	if strings.Contains(v15Metadata, "review") {
		t.Fatalf("a false review flag was not canonically omitted: %s", v15Metadata)
	}
	parsed, err := ParseV15TaskMetadata(v15Metadata)
	if err != nil {
		t.Fatalf("ParseV15TaskMetadata: %v", err)
	}
	if parsed.Review {
		t.Fatal("an omitted review key parsed as flagged")
	}
}

// TestV14RejectsReviewMetadata keeps schema 14 strict and migration-only: it must
// never learn to read the key schema 15 introduced, or a downgraded reader would
// silently drop the flag and with it the lane.
func TestV14RejectsReviewMetadata(t *testing.T) {
	metadata, err := WriteV15TaskMetadata(v15TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseV14TaskMetadata(metadata)
	if err == nil || !strings.Contains(err.Error(), "unknown v14 metadata field \"review\"") {
		t.Fatalf("ParseV14TaskMetadata error = %v, want an unknown review field refusal", err)
	}
	if v14MetadataKeys[v15ReviewMetadataKey] {
		t.Fatal("schema 14 admitted the review key into its metadata key set")
	}
}

// TestV15MalformedReviewMetadataIsRefused pins the one value the key may carry.
// The writer omits the key when the flag is off, so anything but "true" is a
// malformed record rather than a false reading.
func TestV15MalformedReviewMetadataIsRefused(t *testing.T) {
	valid, err := WriteV15TaskMetadata(V15TaskRecord{V14TaskRecord: v14TestRecord()})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "false", "True", "1", "yes"} {
		comment := strings.TrimSuffix(valid, " -->") + ", review: " + value + " -->"
		if _, err := ParseV15TaskMetadata(comment); err == nil {
			t.Fatalf("ParseV15TaskMetadata(%q) succeeded, want refusal", comment)
		}
	}
}

func TestV15WriterAndValidatorRefuseAnUnpersistableRecord(t *testing.T) {
	record := V15TaskRecord{V14TaskRecord: v14TestRecord(), Review: true}
	record.Dependencies = []V14TaskDependency{{ProjectID: "project-alpha", TaskID: ""}}
	if _, err := WriteV15TaskMetadata(record); err == nil {
		t.Fatal("writer accepted a dependency with an empty task id")
	}
	if err := ValidateV15TaskRecord(record); err == nil {
		t.Fatal("validator accepted a dependency with an empty task id")
	}
	untitled := V15TaskRecord{V14TaskRecord: v14TestRecord()}
	untitled.Title = ""
	if err := ValidateV15TaskRecord(untitled); err == nil {
		t.Fatal("validator accepted a record with no title")
	}
	if _, err := WriteV15TaskBody(untitled); err == nil {
		t.Fatal("body writer accepted a record with no title")
	}
}

func TestV15TaskBodyRefusesMissingMetadata(t *testing.T) {
	if _, err := ParseV15TaskBody("Task", "\nno metadata here\n"); err == nil {
		t.Fatal("ParseV15TaskBody accepted a body with no metadata comment")
	}
	if _, err := ParseV15TaskBody("Task", "<!-- tt: id: 1 -->\n"); err == nil {
		t.Fatal("ParseV15TaskBody accepted a malformed metadata comment")
	}
}
