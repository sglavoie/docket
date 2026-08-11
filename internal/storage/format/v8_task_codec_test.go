package format

import (
	"strings"
	"testing"
	"time"
)

func v8TestRecord() V8TaskRecord {
	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 4, 2, 10, 30, 0, 0, time.UTC)
	stateChanged := time.Date(2026, 3, 20, 8, 15, 0, 0, time.UTC)
	return V8TaskRecord{
		V6TaskRecord: V6TaskRecord{
			ID: "task-1", ShortID: 1, Title: "Ship the state clock",
			Description: "Body text", State: "waiting", Priority: 2,
			Tags: []string{"infra"}, CreatedAt: created, UpdatedAt: updated,
			Comments: []V6TaskComment{{Text: "note", CreatedAt: updated}},
		},
		StateChangedAt: stateChanged,
	}
}

func TestV8TaskMetadataRoundTripsTheStateClock(t *testing.T) {
	record := v8TestRecord()
	metadata, err := WriteV8TaskMetadata(record)
	if err != nil {
		t.Fatalf("WriteV8TaskMetadata: %v", err)
	}
	if !strings.Contains(metadata, "updated: 2026-04-02T10:30:00Z, state_changed: 2026-03-20T08:15:00Z") {
		t.Fatalf("state clock is not written next to the other timestamps: %s", metadata)
	}
	parsed, err := ParseV8TaskMetadata(metadata)
	if err != nil {
		t.Fatalf("ParseV8TaskMetadata: %v", err)
	}
	if !parsed.StateChangedAt.Equal(record.StateChangedAt) {
		t.Fatalf("state clock = %v, want %v", parsed.StateChangedAt, record.StateChangedAt)
	}
	if !parsed.UpdatedAt.Equal(record.UpdatedAt) {
		t.Fatalf("updated = %v, want %v", parsed.UpdatedAt, record.UpdatedAt)
	}
}

func TestV8TaskBodyRoundTrip(t *testing.T) {
	record := v8TestRecord()
	body, err := WriteV8TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV8TaskBody: %v", err)
	}
	parsed, err := ParseV8TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV8TaskBody: %v", err)
	}
	if parsed.Description != record.Description || len(parsed.Comments) != 1 {
		t.Fatalf("body round trip lost content: %+v", parsed)
	}
	if !parsed.StateChangedAt.Equal(record.StateChangedAt) {
		t.Fatalf("state clock = %v, want %v", parsed.StateChangedAt, record.StateChangedAt)
	}
}

func TestV8TaskMetadataRequiresTheStateClock(t *testing.T) {
	record := v8TestRecord()
	record.StateChangedAt = time.Time{}
	if _, err := WriteV8TaskMetadata(record); err == nil ||
		!strings.Contains(err.Error(), "requires a state changed timestamp") {
		t.Fatalf("writer error = %v, want a missing state clock rejection", err)
	}

	valid, err := WriteV8TaskMetadata(v8TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	withoutClock := strings.Replace(valid, ", state_changed: 2026-03-20T08:15:00Z", "", 1)
	if _, err := ParseV8TaskMetadata(withoutClock); err == nil ||
		!strings.Contains(err.Error(), `missing required v8 metadata field "state_changed"`) {
		t.Fatalf("parser error = %v, want a missing state clock rejection", err)
	}
}

func TestV8TaskMetadataRejectsSchema7Comment(t *testing.T) {
	legacy, err := WriteV6TaskMetadata(v8TestRecord().V6TaskRecord)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV8TaskMetadata(legacy); err == nil {
		t.Fatal("schema-8 parser accepted a schema-7 metadata comment")
	}
}

func TestV8TaskRecordFromV7BackfillsFromUpdated(t *testing.T) {
	legacy := v8TestRecord().V6TaskRecord
	lifted := V8TaskRecordFromV7(legacy)
	if !lifted.StateChangedAt.Equal(legacy.UpdatedAt) {
		t.Fatalf("backfilled state clock = %v, want %v", lifted.StateChangedAt, legacy.UpdatedAt)
	}
}
