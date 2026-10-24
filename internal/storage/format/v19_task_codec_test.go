package format

import (
	"strings"
	"testing"
)

// TestV19TaskBodyRoundTripsTheHiddenFlag writes the flag as the last metadata
// key, reads it back, and reproduces the same bytes on a rewrite.
func TestV19TaskBodyRoundTripsTheHiddenFlag(t *testing.T) {
	record := V19TaskRecord{V18TaskRecord: v18TestRecord(), Hidden: true}
	body, err := WriteV19TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV19TaskBody: %v", err)
	}
	if !strings.Contains(body, "reviewed: 2026-03-01T09:15:00Z, hidden: true -->") {
		t.Fatalf("metadata does not end with the hidden flag:\n%s", body)
	}
	parsed, err := ParseV19TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV19TaskBody: %v", err)
	}
	if !parsed.Hidden {
		t.Fatal("hidden flag did not survive the round trip")
	}
	rewritten, err := WriteV19TaskBody(parsed)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if rewritten != body {
		t.Fatalf("encoding is not deterministic\nfirst:\n%s\nsecond:\n%s", body, rewritten)
	}
}

// TestV19TaskBodyWithoutTheFlagIsTheSchema18Body is the additive discipline: a
// task that is not hidden writes exactly the bytes schema 18 wrote.
func TestV19TaskBodyWithoutTheFlagIsTheSchema18Body(t *testing.T) {
	base := v18TestRecord()
	v18, err := WriteV18TaskBody(base)
	if err != nil {
		t.Fatal(err)
	}
	v19, err := WriteV19TaskBody(V19TaskRecord{V18TaskRecord: base})
	if err != nil {
		t.Fatal(err)
	}
	if v18 != v19 {
		t.Fatalf("an unhidden task changed bytes\nv18:\n%s\nv19:\n%s", v18, v19)
	}
	parsed, err := ParseV19TaskBody(base.Title, v18)
	if err != nil {
		t.Fatalf("ParseV19TaskBody(schema-18 body): %v", err)
	}
	if parsed.Hidden {
		t.Fatal("an absent key parsed as hidden")
	}
}

// TestV19TaskMetadataRefusesANonCanonicalHiddenValue keeps parsing strict: the
// writer omits the key when the flag is off, so any spelling but "true" is a
// file this writer did not produce.
func TestV19TaskMetadataRefusesANonCanonicalHiddenValue(t *testing.T) {
	body, err := WriteV19TaskBody(V19TaskRecord{V18TaskRecord: v18TestRecord(), Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{"false", "yes", "TRUE", ""} {
		mutated := strings.Replace(body, "hidden: true", "hidden: "+spelling, 1)
		if _, parseErr := ParseV19TaskBody("Title", mutated); parseErr == nil {
			t.Errorf("hidden: %q parsed", spelling)
		}
	}
	if _, err := ParseV18TaskBody("Title", body); err == nil {
		t.Error("the frozen schema-18 reader accepted a hidden key")
	}
}
