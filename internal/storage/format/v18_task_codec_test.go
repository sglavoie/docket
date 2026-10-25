package format

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func v17TestRecord() V17TaskRecord {
	return V17TaskRecord{
		V16TaskRecord:   v16TestRecord(),
		ChecklistOrigin: &models.ChecklistOrigin{ProjectID: "project-alpha", TaskID: "task-parent", Index: 3},
	}
}

func v18TestRecord() V18TaskRecord {
	reviewed := time.Date(2026, 3, 1, 9, 15, 0, 0, time.UTC)
	return V18TaskRecord{
		V17TaskRecord:     v17TestRecord(),
		NextCheckIn:       "2026-03-15",
		CheckInRepeatDays: 14,
		LastReviewedAt:    &reviewed,
	}
}

// TestV18TaskBodyRoundTripsEveryCheckInValue is the whole point of the record:
// all three values survive, in canonical order, and re-writing what was parsed
// reproduces the same bytes — a load/save with no user edit must not churn the
// file or every stored revision would move.
func TestV18TaskBodyRoundTripsEveryCheckInValue(t *testing.T) {
	record := v18TestRecord()
	body, err := WriteV18TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV18TaskBody: %v", err)
	}
	if !strings.Contains(body, "check_in: 2026-03-15, check_in_repeat: 14, reviewed: 2026-03-01T09:15:00Z -->") {
		t.Fatalf("metadata does not carry the canonical check-in tail:\n%s", body)
	}
	parsed, err := ParseV18TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV18TaskBody: %v", err)
	}
	if parsed.NextCheckIn != record.NextCheckIn || parsed.CheckInRepeatDays != record.CheckInRepeatDays {
		t.Fatalf("check-in = %q/%d, want %q/%d",
			parsed.NextCheckIn, parsed.CheckInRepeatDays, record.NextCheckIn, record.CheckInRepeatDays)
	}
	if parsed.LastReviewedAt == nil || !parsed.LastReviewedAt.Equal(*record.LastReviewedAt) {
		t.Fatalf("review instant = %v, want %v", parsed.LastReviewedAt, record.LastReviewedAt)
	}
	rewritten, err := WriteV18TaskBody(parsed)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if rewritten != body {
		t.Fatalf("encoding is not deterministic\nfirst:\n%s\nsecond:\n%s", body, rewritten)
	}
}

// TestV18TaskBodyKeepsEveryInheritedBlock is the "changes the metadata line
// alone" claim, checked rather than asserted in a comment: the origin line, the
// resume note, the description, the comments, and the inherited metadata keys
// all have to come back untouched.
func TestV18TaskBodyKeepsEveryInheritedBlock(t *testing.T) {
	record := v18TestRecord()
	body, err := WriteV18TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV18TaskBody: %v", err)
	}
	parsed, err := ParseV18TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV18TaskBody: %v", err)
	}
	if !reflect.DeepEqual(parsed.ChecklistOrigin, record.ChecklistOrigin) {
		t.Errorf("checklist origin = %+v, want %+v", parsed.ChecklistOrigin, record.ChecklistOrigin)
	}
	if parsed.ResumeNote != record.ResumeNote {
		t.Errorf("resume note = %q, want %q", parsed.ResumeNote, record.ResumeNote)
	}
	if parsed.Description != record.Description || len(parsed.Comments) != len(record.Comments) {
		t.Errorf("body blocks changed: %+v", parsed)
	}
	if !parsed.Review || parsed.Context != record.Context || parsed.DueTime != record.DueTime ||
		!reflect.DeepEqual(parsed.Dependencies, record.Dependencies) {
		t.Errorf("inherited metadata changed: %+v", parsed)
	}
}

// TestV18AbsentCheckInMatchesV17Bytes is the byte-equality guarantee the
// schema-19 document rests on: a task with no schedule and no review history
// must serialize exactly as its schema-17 predecessor did, so lifting a whole
// workspace rewrites the marker line and nothing else.
func TestV18AbsentCheckInMatchesV17Bytes(t *testing.T) {
	base := v17TestRecord()
	v17Body, err := WriteV17TaskBody(base)
	if err != nil {
		t.Fatal(err)
	}
	v18Body, err := WriteV18TaskBody(V18TaskRecord{V17TaskRecord: base})
	if err != nil {
		t.Fatal(err)
	}
	if v18Body != v17Body {
		t.Fatalf("an unscheduled task changed inherited bytes\nv17:\n%s\nv18:\n%s", v17Body, v18Body)
	}
	for _, key := range []string{"check_in", "check_in_repeat", "reviewed"} {
		if strings.Contains(v18Body, key) {
			t.Fatalf("an absent %s was not canonically omitted:\n%s", key, v18Body)
		}
	}
	parsed, err := ParseV18TaskBody(base.Title, v18Body)
	if err != nil {
		t.Fatalf("ParseV18TaskBody: %v", err)
	}
	if parsed.NextCheckIn != "" || parsed.CheckInRepeatDays != 0 || parsed.LastReviewedAt != nil {
		t.Fatalf("omitted keys parsed as a schedule: %+v", parsed)
	}
}

// TestV18CarriesReviewHistoryWithoutASchedule is the absent-versus-cleared
// distinction at the storage boundary. A completed task keeps what happened and
// loses what is owed, so the record has to be able to spell a review instant
// with no date and no repeat beside it.
func TestV18CarriesReviewHistoryWithoutASchedule(t *testing.T) {
	reviewed := time.Date(2026, 3, 1, 9, 15, 0, 0, time.UTC)
	record := V18TaskRecord{V17TaskRecord: v17TestRecord(), LastReviewedAt: &reviewed}
	body, err := WriteV18TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV18TaskBody: %v", err)
	}
	if strings.Contains(body, "check_in") {
		t.Fatalf("a cleared schedule wrote a check-in key:\n%s", body)
	}
	parsed, err := ParseV18TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV18TaskBody: %v", err)
	}
	if parsed.NextCheckIn != "" || parsed.CheckInRepeatDays != 0 {
		t.Errorf("review history revived a schedule: %+v", parsed)
	}
	if parsed.LastReviewedAt == nil || !parsed.LastReviewedAt.Equal(reviewed) {
		t.Errorf("review instant = %v, want %v", parsed.LastReviewedAt, reviewed)
	}
}

// TestV18OneOffCheckInNeedsNoRepeat pins the other legal pairing: a date with
// the repeat off is a check-in that happens once.
func TestV18OneOffCheckInNeedsNoRepeat(t *testing.T) {
	record := V18TaskRecord{V17TaskRecord: v17TestRecord(), NextCheckIn: "2026-03-15"}
	body, err := WriteV18TaskBody(record)
	if err != nil {
		t.Fatalf("WriteV18TaskBody: %v", err)
	}
	if strings.Contains(body, "check_in_repeat") {
		t.Fatalf("an off repeat was written:\n%s", body)
	}
	parsed, err := ParseV18TaskBody(record.Title, body)
	if err != nil {
		t.Fatalf("ParseV18TaskBody: %v", err)
	}
	if parsed.NextCheckIn != "2026-03-15" || parsed.CheckInRepeatDays != models.CheckInRepeatOff {
		t.Fatalf("one-off check-in did not round trip: %+v", parsed)
	}
}

// TestV18WriterRefusesUnrepresentableSchedules keeps the two halves in
// agreement: the writer must never produce a record the reader refuses, and the
// bound it applies is the model's rather than one of its own.
func TestV18WriterRefusesUnrepresentableSchedules(t *testing.T) {
	cases := map[string]func(*V18TaskRecord){
		"repeat with no date":   func(r *V18TaskRecord) { r.NextCheckIn = ""; r.CheckInRepeatDays = 7 },
		"negative repeat":       func(r *V18TaskRecord) { r.CheckInRepeatDays = -1 },
		"repeat past the bound": func(r *V18TaskRecord) { r.CheckInRepeatDays = models.MaxCheckInRepeatDays + 1 },
		"noncanonical date":     func(r *V18TaskRecord) { r.NextCheckIn = "2026-3-15" },
		"date with a clock":     func(r *V18TaskRecord) { r.NextCheckIn = "2026-03-15 09:00" },
		"zero review instant":   func(r *V18TaskRecord) { r.LastReviewedAt = &time.Time{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			record := v18TestRecord()
			mutate(&record)
			if _, err := WriteV18TaskBody(record); err == nil {
				t.Fatalf("the writer accepted an unrepresentable schedule: %+v", record)
			}
		})
	}
}

// TestV18ReaderRefusesShapesTheWriterCannotProduce covers the metadata values
// the canonical writer never emits, so accepting one would mean guessing at a
// file something else wrote.
func TestV18ReaderRefusesShapesTheWriterCannotProduce(t *testing.T) {
	base, err := WriteV18TaskMetadata(V18TaskRecord{V17TaskRecord: v17TestRecord()})
	if err != nil {
		t.Fatal(err)
	}
	for name, tail := range map[string]string{
		"repeat with no date":      "check_in_repeat: 7",
		"repeat written off":       "check_in: 2026-03-15, check_in_repeat: 0",
		"unpadded date":            "check_in: 2026-3-15",
		"repeat is not a number":   "check_in: 2026-03-15, check_in_repeat: weekly",
		"review instant is a date": "reviewed: 2026-03-01",
	} {
		t.Run(name, func(t *testing.T) {
			comment := strings.TrimSuffix(base, " -->") + ", " + tail + " -->"
			if _, err := ParseV18TaskMetadata(comment); err == nil {
				t.Fatalf("the reader accepted %s", comment)
			}
		})
	}
}

// TestV17RejectsSchema18Metadata keeps schema 17 frozen. A historical reader
// that learned a newer key would stop being a fixed target for the archives a
// migration validates against.
func TestV17RejectsSchema18Metadata(t *testing.T) {
	body, err := WriteV18TaskBody(v18TestRecord())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseV17TaskBody("Check-in", body); err == nil {
		t.Fatalf("schema 17 accepted schema-18 metadata:\n%s", body)
	}
}

// TestV18DateIsStoredAsWrittenRegardlessOfZone pins the calendar-date semantics
// at the boundary: the stored spelling is the date itself, so reading it back
// cannot depend on the machine's zone the way an instant would.
func TestV18DateIsStoredAsWrittenRegardlessOfZone(t *testing.T) {
	apia, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Skipf("zone database unavailable: %v", err)
	}
	record := v18TestRecord()
	body, err := WriteV18TaskBody(record)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseV18TaskBody(record.Title, body)
	if err != nil {
		t.Fatal(err)
	}
	day, err := time.ParseInLocation("2006-01-02", parsed.NextCheckIn, apia)
	if err != nil {
		t.Fatalf("stored date does not parse in another zone: %v", err)
	}
	if year, month, dayOfMonth := day.Date(); year != 2026 || month != time.March || dayOfMonth != 15 {
		t.Fatalf("stored date read back as %s in %s", day, apia)
	}
}
