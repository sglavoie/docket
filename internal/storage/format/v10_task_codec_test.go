package format

import (
	"strings"
	"testing"
)

func v10TestRecord() V10TaskRecord {
	record := v8TestRecord()
	record.SnoozedUntil = "2026-05-14T09:00:00Z"
	return record
}

func TestV10TaskMetadataRoundTripsTheDeferralInstant(t *testing.T) {
	record := v10TestRecord()
	metadata, err := WriteV10TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV10TaskMetadata: %v", err)
	}
	if !strings.Contains(metadata, "deferred: 2026-05-14T09:00:00Z") {
		t.Fatalf("metadata is missing the deferral instant: %s", metadata)
	}
	parsed, err := ParseV10TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV10TaskMetadata: %v", err)
	}
	if parsed.SnoozedUntil != record.SnoozedUntil {
		t.Fatalf("deferral instant = %q, want %q", parsed.SnoozedUntil, record.SnoozedUntil)
	}
	if !parsed.StateChangedAt.Equal(record.StateChangedAt) {
		t.Fatalf("state clock = %v, want %v", parsed.StateChangedAt, record.StateChangedAt)
	}
}

func TestV10TaskBodyRoundTrip(t *testing.T) {
	record := v10TestRecord()
	body, err := WriteV10TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV10TaskBody: %v", err)
	}
	parsed, err := ParseV10TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV10TaskBody: %v", err)
	}
	if parsed.Description != record.Description || len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
	if parsed.SnoozedUntil != record.SnoozedUntil {
		t.Fatalf("deferral instant = %q, want %q", parsed.SnoozedUntil, record.SnoozedUntil)
	}
}

// TestV10TaskMetadataRejectsABareDate and its schema-9 twin below are what make
// a half-applied layout change loud. Schema 9 and schema 10 write the deferral
// value under the same key, so only the value check can tell them apart.
func TestV10TaskMetadataRejectsABareDate(t *testing.T) {
	record := v10TestRecord()
	record.SnoozedUntil = "2026-05-14"
	if _, err := WriteV10TaskMetadata(record); err == nil ||
		!strings.Contains(err.Error(), `deferred date "2026-05-14" is not canonical RFC3339Nano`) {
		t.Fatalf("writer error = %v, want a bare deferral date rejection", err)
	}

	legacy, err := WriteV9TaskMetadata(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV10TaskMetadata(legacy); err == nil ||
		!strings.Contains(err.Error(), `deferred date "2026-05-14" is not canonical RFC3339Nano`) {
		t.Fatalf("parser error = %v, want a bare deferral date rejection", err)
	}
}

func TestV9TaskMetadataRejectsADeferralInstant(t *testing.T) {
	record := v10TestRecord()
	if _, err := WriteV9TaskMetadata(record); err == nil ||
		!strings.Contains(err.Error(), `deferred date "2026-05-14T09:00:00Z" is not canonical YYYY-MM-DD`) {
		t.Fatalf("writer error = %v, want a deferral instant rejection", err)
	}

	current, err := WriteV10TaskMetadata(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV9TaskMetadata(current); err == nil ||
		!strings.Contains(err.Error(), `deferred date "2026-05-14T09:00:00Z" is not canonical YYYY-MM-DD`) {
		t.Fatalf("parser error = %v, want a deferral instant rejection", err)
	}
}

// The write, parse, and full-record paths all reach one validator, and only the
// encoding it is handed says which metadata field the user must repair and in
// which layout.
func TestV10TaskDeferralInstantErrorsNameTheDeferredKey(t *testing.T) {
	invalid := v10TestRecord()
	invalid.SnoozedUntil = "2026-05-14T09:00:00.000Z"

	assertDeferredVocabulary := func(t *testing.T, label string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s accepted a noncanonical deferral instant", label)
		}
		if !strings.Contains(err.Error(), `deferred date "2026-05-14T09:00:00.000Z" is not canonical RFC3339Nano`) {
			t.Fatalf("%s error = %v, want the deferred key named", label, err)
		}
		if strings.Contains(err.Error(), "snoozed") {
			t.Fatalf("%s error = %v, want no retired vocabulary", label, err)
		}
	}

	_, err := WriteV10TaskMetadata(invalid)
	assertDeferredVocabulary(t, "WriteV10TaskMetadata", err)
	_, err = WriteV10TaskBody(invalid)
	assertDeferredVocabulary(t, "WriteV10TaskBody", err)
	assertDeferredVocabulary(t, "ValidateV10TaskRecord", ValidateV10TaskRecord(invalid))

	valid, err := WriteV10TaskMetadata(v10TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseV10TaskMetadata(strings.Replace(valid,
		"deferred: 2026-05-14T09:00:00Z", "deferred: 2026-05-14T09:00:00.000Z", 1))
	assertDeferredVocabulary(t, "ParseV10TaskMetadata", err)
}
