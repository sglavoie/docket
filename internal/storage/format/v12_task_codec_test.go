package format

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func v12TestRecord() V12TaskRecord {
	return V12TaskRecord{V11TaskRecord: v11TestRecord(), DeferCount: 3}
}

func TestV12TaskMetadataRoundTripsOptionalDeferCountLast(t *testing.T) {
	record := v12TestRecord()
	metadata, err := WriteV12TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV12TaskMetadata: %v", err)
	}
	if !strings.HasSuffix(metadata, "context: platform-api, defer_count: 3 -->") {
		t.Fatalf("defer_count is not appended after context: %s", metadata)
	}
	parsed, err := ParseV12TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV12TaskMetadata: %v", err)
	}
	if parsed.DeferCount != record.DeferCount {
		t.Fatalf("defer count = %d, want %d", parsed.DeferCount, record.DeferCount)
	}
}

func TestV12TaskBodyRoundTrip(t *testing.T) {
	record := v12TestRecord()
	body, err := WriteV12TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV12TaskBody: %v", err)
	}
	parsed, err := ParseV12TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV12TaskBody: %v", err)
	}
	if parsed.DeferCount != record.DeferCount || parsed.Context != record.Context ||
		parsed.Description != record.Description || len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
}

func TestV12DeferCountIsOptionalAndRequiredMetadataMatchesV11(t *testing.T) {
	if !reflect.DeepEqual(v12RequiredMetadataKeys, v11RequiredMetadataKeys) {
		t.Fatalf("required metadata changed: v12=%v v11=%v", v12RequiredMetadataKeys, v11RequiredMetadataKeys)
	}
	record := v11TestRecord()
	v11Metadata, err := WriteV11TaskMetadata(record)
	if err != nil {
		t.Fatal(err)
	}
	v12Metadata, err := WriteV12TaskMetadata(V12TaskRecord{V11TaskRecord: record})
	if err != nil {
		t.Fatal(err)
	}
	if v12Metadata != v11Metadata {
		t.Fatalf("schema-12 zero-count encoding changed inherited bytes\nv11: %s\nv12: %s", v11Metadata, v12Metadata)
	}
	parsed, err := ParseV12TaskMetadata(v12Metadata)
	if err != nil {
		t.Fatalf("ParseV12TaskMetadata: %v", err)
	}
	if parsed.DeferCount != 0 {
		t.Fatalf("parsed defer count = %d, want zero", parsed.DeferCount)
	}
}

func TestV12DeferCountRejectsInvalidPersistedValues(t *testing.T) {
	record := v12TestRecord()
	record.DeferCount = -1
	if _, err := WriteV12TaskMetadata(record); err == nil || !strings.Contains(err.Error(), "negative defer count") {
		t.Fatalf("negative count write error = %v", err)
	}

	valid, err := WriteV12TaskMetadata(V12TaskRecord{V11TaskRecord: v11TestRecord()})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "0", "-1", "1.5", "1:bad", strconv.FormatUint(^uint64(0), 10)} {
		metadata := strings.Replace(valid, " -->", ", defer_count: "+value+" -->", 1)
		if _, err := ParseV12TaskMetadata(metadata); err == nil {
			t.Fatalf("defer_count %q was accepted", value)
		}
	}
}
