package format

import (
	"reflect"
	"strings"
	"testing"
)

func v13TestRecord() V13TaskRecord {
	record := V13TaskRecord{V12TaskRecord: v12TestRecord(), DueTime: "15:30"}
	record.DueDate = "2026-08-25"
	return record
}

func TestV13TaskMetadataRoundTripsOptionalDueTimeLast(t *testing.T) {
	record := v13TestRecord()
	metadata, err := WriteV13TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV13TaskMetadata: %v", err)
	}
	if !strings.HasSuffix(metadata, "due_time: 15:30 -->") {
		t.Fatalf("due_time is not appended last: %s", metadata)
	}
	parsed, err := ParseV13TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV13TaskMetadata: %v", err)
	}
	if parsed.DueTime != record.DueTime {
		t.Fatalf("due_time = %q, want %q", parsed.DueTime, record.DueTime)
	}
}

func TestV13TaskBodyRoundTrip(t *testing.T) {
	record := v13TestRecord()
	body, err := WriteV13TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV13TaskBody: %v", err)
	}
	parsed, err := ParseV13TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV13TaskBody: %v", err)
	}
	if parsed.DueTime != record.DueTime || parsed.DeferCount != record.DeferCount ||
		parsed.Context != record.Context || parsed.Description != record.Description ||
		len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
}

func TestV13DueTimeIsOptionalAndMatchesV12WhenAbsent(t *testing.T) {
	if !reflect.DeepEqual(v13RequiredMetadataKeys, v12RequiredMetadataKeys) {
		t.Fatalf("required metadata changed: v13=%v v12=%v", v13RequiredMetadataKeys, v12RequiredMetadataKeys)
	}
	base := v12TestRecord()
	v12Metadata, err := WriteV12TaskMetadata(base)
	if err != nil {
		t.Fatal(err)
	}
	v13Metadata, err := WriteV13TaskMetadata(V13TaskRecord{V12TaskRecord: base})
	if err != nil {
		t.Fatal(err)
	}
	if v13Metadata != v12Metadata {
		t.Fatalf("schema-13 date-only encoding changed inherited bytes\nv12: %s\nv13: %s", v12Metadata, v13Metadata)
	}
	parsed, err := ParseV13TaskMetadata(v13Metadata)
	if err != nil {
		t.Fatalf("ParseV13TaskMetadata: %v", err)
	}
	if parsed.DueTime != "" {
		t.Fatalf("parsed due_time = %q, want empty", parsed.DueTime)
	}
}

func TestV13DueTimeRequiresADueDate(t *testing.T) {
	record := v13TestRecord()
	record.DueDate = ""
	if _, err := WriteV13TaskMetadata(record); err == nil || !strings.Contains(err.Error(), "without a due date") {
		t.Fatalf("due_time without due date error = %v", err)
	}
}

func TestV13DueTimeRejectsNonCanonicalValues(t *testing.T) {
	valid, err := WriteV13TaskMetadata(V13TaskRecord{V12TaskRecord: v12TestRecord()})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"9:30", "24:00", "12:60", "1230", "ab:cd", "00:00", "12:5"} {
		metadata := strings.Replace(valid, " -->", ", due_time: "+value+" -->", 1)
		if _, err := ParseV13TaskMetadata(metadata); err == nil {
			t.Fatalf("due_time %q was accepted", value)
		}
	}
}

func TestValidateV13TaskRecordAcceptsCanonicalTimes(t *testing.T) {
	for _, value := range []string{"", "00:01", "09:00", "23:59"} {
		record := v13TestRecord()
		record.DueDate = "2026-08-25"
		record.DueTime = value
		if err := ValidateV13TaskRecord(record); err != nil {
			t.Fatalf("ValidateV13TaskRecord(%q) = %v", value, err)
		}
	}
}
