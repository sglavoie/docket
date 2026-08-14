package format

import (
	"strings"
	"testing"
	"time"
)

func v9TestRecord() V9TaskRecord {
	record := v8TestRecord()
	record.SnoozedUntil = "2026-05-14"
	return record
}

func TestV9TaskMetadataWritesTheDeferredKey(t *testing.T) {
	record := v9TestRecord()
	metadata, err := WriteV9TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV9TaskMetadata: %v", err)
	}
	if !strings.Contains(metadata, "deferred: 2026-05-14") {
		t.Fatalf("metadata is missing the deferred key: %s", metadata)
	}
	if strings.Contains(metadata, "snoozed:") {
		t.Fatalf("metadata still carries the legacy key: %s", metadata)
	}
	parsed, err := ParseV9TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV9TaskMetadata: %v", err)
	}
	if parsed.SnoozedUntil != record.SnoozedUntil {
		t.Fatalf("deferral date = %q, want %q", parsed.SnoozedUntil, record.SnoozedUntil)
	}
	if !parsed.StateChangedAt.Equal(record.StateChangedAt) {
		t.Fatalf("state clock = %v, want %v", parsed.StateChangedAt, record.StateChangedAt)
	}
}

// TestV9TaskMetadataRejectsTheLegacyKey and its schema-8 twin below are what make
// a half-applied rename loud. The key is optional, so a parser that accepted the
// other schema's spelling would drop every deferral date without an error.
func TestV9TaskMetadataRejectsTheLegacyKey(t *testing.T) {
	legacy, err := WriteV8TaskMetadata(v9TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(legacy, "snoozed: 2026-05-14") {
		t.Fatalf("schema-8 metadata is missing the legacy key: %s", legacy)
	}
	if _, err := ParseV9TaskMetadata(legacy); err == nil ||
		!strings.Contains(err.Error(), `unknown v9 metadata field "snoozed"`) {
		t.Fatalf("parser error = %v, want a legacy deferral key rejection", err)
	}
}

func TestV8TaskMetadataRejectsTheDeferredKey(t *testing.T) {
	current, err := WriteV9TaskMetadata(v9TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV8TaskMetadata(current); err == nil ||
		!strings.Contains(err.Error(), `unknown v8 metadata field "deferred"`) {
		t.Fatalf("parser error = %v, want a schema-9 deferral key rejection", err)
	}
}

func TestV9TaskBodyRoundTrip(t *testing.T) {
	record := v9TestRecord()
	body, err := WriteV9TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV9TaskBody: %v", err)
	}
	parsed, err := ParseV9TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV9TaskBody: %v", err)
	}
	if parsed.Description != record.Description || len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
	if parsed.SnoozedUntil != record.SnoozedUntil {
		t.Fatalf("deferral date = %q, want %q", parsed.SnoozedUntil, record.SnoozedUntil)
	}
}

func TestV9TaskMetadataOmitsAnUndeferredTask(t *testing.T) {
	record := v9TestRecord()
	record.SnoozedUntil = ""
	metadata, err := WriteV9TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV9TaskMetadata: %v", err)
	}
	if strings.Contains(metadata, "deferred:") {
		t.Fatalf("metadata carries an empty deferral date: %s", metadata)
	}
	parsed, err := ParseV9TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV9TaskMetadata: %v", err)
	}
	if parsed.SnoozedUntil != "" {
		t.Fatalf("deferral date = %q, want empty", parsed.SnoozedUntil)
	}
}

// The write, parse, and full-record paths all reach one validator, and only the
// key it is handed says which metadata field the user must repair.
func TestV9TaskDeferralDateErrorsNameTheDeferredKey(t *testing.T) {
	invalid := v9TestRecord()
	invalid.SnoozedUntil = "2026-5-14"

	assertDeferredVocabulary := func(t *testing.T, label string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s accepted a noncanonical deferral date", label)
		}
		if !strings.Contains(err.Error(), `deferred date "2026-5-14" is not canonical`) {
			t.Fatalf("%s error = %v, want the deferred key named", label, err)
		}
		if strings.Contains(err.Error(), "snoozed") {
			t.Fatalf("%s error = %v, want no retired vocabulary", label, err)
		}
	}

	_, err := WriteV9TaskMetadata(invalid)
	assertDeferredVocabulary(t, "WriteV9TaskMetadata", err)
	_, err = WriteV9TaskBody(invalid)
	assertDeferredVocabulary(t, "WriteV9TaskBody", err)
	assertDeferredVocabulary(t, "ValidateV9TaskRecord", ValidateV9TaskRecord(invalid))

	valid, err := WriteV9TaskMetadata(v9TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseV9TaskMetadata(strings.Replace(valid, "deferred: 2026-05-14", "deferred: 2026-5-14", 1))
	assertDeferredVocabulary(t, "ParseV9TaskMetadata", err)
}

// Schema 6 and schema 8 write `snoozed` on disk, so their errors keep naming it.
func TestLegacyDeferralDateErrorsKeepTheSnoozedKey(t *testing.T) {
	invalid := v9TestRecord()
	invalid.SnoozedUntil = "2026-5-14"

	for _, testCase := range []struct {
		name  string
		write func() (string, error)
	}{
		{"WriteV8TaskMetadata", func() (string, error) { return WriteV8TaskMetadata(invalid) }},
		{"WriteV6TaskMetadata", func() (string, error) { return WriteV6TaskMetadata(invalid.V6TaskRecord) }},
	} {
		_, err := testCase.write()
		if err == nil || !strings.Contains(err.Error(), `snoozed date "2026-5-14" is not canonical`) {
			t.Fatalf("%s error = %v, want the legacy snoozed key named", testCase.name, err)
		}
	}
}

func TestV9TaskMetadataRequiresTheStateClock(t *testing.T) {
	record := v9TestRecord()
	record.StateChangedAt = time.Time{}
	if _, err := WriteV9TaskMetadata(record); err == nil ||
		!strings.Contains(err.Error(), "requires a state changed timestamp") {
		t.Fatalf("writer error = %v, want a missing state clock rejection", err)
	}
}
