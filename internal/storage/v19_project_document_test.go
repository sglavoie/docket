package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage/format"
)

func v19ProjectTestDocument(t *testing.T) V19ProjectDocument {
	t.Helper()
	record, err := currentTaskRecordFromModel(mergeTestTask("task-1", "Lifted"))
	if err != nil {
		t.Fatal(err)
	}
	return V19ProjectDocument{
		ProjectName: "Check-ins",
		Tasks:       []format.V18TaskRecord{record.V18TaskRecord},
	}
}

// TestV19ProjectDocumentLiftFromV18IsTheMarkerAlone is what makes the schema-19
// migration cheap and reviewable: no retired field carried a check-in schedule
// or a review instant into this boundary, so every lifted task has all three
// absent, and a document with them absent has to render exactly the bytes its
// schema-18 source did apart from the marker line.
func TestV19ProjectDocumentLiftFromV18IsTheMarkerAlone(t *testing.T) {
	source := V18ProjectDocument{
		ProjectName: "Check-ins",
		Comments:    []format.ProjectCommentRecord{v17TestComment(v17TestCommentIDA, "kept", 0)},
		Tasks:       []format.V17TaskRecord{v19ProjectTestDocument(t).Tasks[0].V17TaskRecord},
	}
	lifted := V19ProjectDocumentFromV18(source)
	for i, task := range lifted.Tasks {
		if task.NextCheckIn != "" || task.CheckInRepeatDays != 0 || task.LastReviewedAt != nil {
			t.Fatalf("lifted task %d invented a schedule: %+v", i, task)
		}
	}
	published, err := WriteV19ProjectDocument(lifted)
	if err != nil {
		t.Fatalf("WriteV19ProjectDocument: %v", err)
	}
	original, err := WriteV18ProjectDocument(source)
	if err != nil {
		t.Fatalf("WriteV18ProjectDocument: %v", err)
	}
	if strings.Replace(published, schemaMarkerV19, schemaMarkerV18, 1) != original {
		t.Fatalf("lift changed bytes beyond the marker\nv18:\n%s\nv19:\n%s", original, published)
	}
}

// TestV19ProjectDocumentRoundTripsCheckInsBesideEveryOtherBlock is the document
// level of the round trip: the schedule has to survive the heading scan, the
// project-comment block, and the project-wide validation, and it has to survive
// them without disturbing anything already stored beside it.
func TestV19ProjectDocumentRoundTripsCheckInsBesideEveryOtherBlock(t *testing.T) {
	reviewed := time.Date(2026, 3, 1, 9, 15, 0, 0, time.UTC)
	document := v19ProjectTestDocument(t)
	document.Comments = []format.ProjectCommentRecord{
		v17TestComment(v17TestCommentIDA, "first note\nsecond line", 0),
		v17TestComment(v17TestCommentIDB, "later note", time.Hour),
	}
	scheduled := document.Tasks[0]
	scheduled.NextCheckIn = "2026-03-15"
	scheduled.CheckInRepeatDays = 7
	scheduled.LastReviewedAt = &reviewed
	scheduled.ResumeNote = "picked this up mid-thought"
	scheduled.Description = "body text"
	document.Tasks = []format.V18TaskRecord{scheduled}

	published, err := WriteV19ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV19ProjectDocument: %v", err)
	}
	parsed, err := ParseV19ProjectDocument(published)
	if err != nil {
		t.Fatalf("ParseV19ProjectDocument: %v", err)
	}
	if len(parsed.Tasks) != 1 {
		t.Fatalf("parsed %d tasks, want 1", len(parsed.Tasks))
	}
	got := parsed.Tasks[0]
	if got.NextCheckIn != scheduled.NextCheckIn || got.CheckInRepeatDays != scheduled.CheckInRepeatDays {
		t.Errorf("schedule = %q/%d, want %q/%d",
			got.NextCheckIn, got.CheckInRepeatDays, scheduled.NextCheckIn, scheduled.CheckInRepeatDays)
	}
	if got.LastReviewedAt == nil || !got.LastReviewedAt.Equal(reviewed) {
		t.Errorf("review instant = %v, want %v", got.LastReviewedAt, reviewed)
	}
	if got.ResumeNote != scheduled.ResumeNote || got.Description != scheduled.Description {
		t.Errorf("task body blocks changed: %+v", got)
	}
	if len(parsed.Comments) != len(document.Comments) {
		t.Fatalf("parsed %d project comments, want %d", len(parsed.Comments), len(document.Comments))
	}
	for i, want := range document.Comments {
		if parsed.Comments[i].ID != want.ID || parsed.Comments[i].Text != want.Text ||
			!parsed.Comments[i].CreatedAt.Equal(want.CreatedAt) {
			t.Fatalf("project comment %d = %+v, want %+v", i, parsed.Comments[i], want)
		}
	}
	rewritten, err := WriteV19ProjectDocument(parsed)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if rewritten != published {
		t.Fatalf("document encoding is not deterministic\nfirst:\n%s\nsecond:\n%s", published, rewritten)
	}
}

// TestV19ProjectDocumentRefusesUnrepresentableSchedules keeps the document
// writer as strict as the record's: a schedule that could not be read back must
// never reach a task file.
func TestV19ProjectDocumentRefusesUnrepresentableSchedules(t *testing.T) {
	for name, mutate := range map[string]func(*format.V18TaskRecord){
		"repeat with no date": func(task *format.V18TaskRecord) { task.CheckInRepeatDays = 7 },
		"noncanonical date":   func(task *format.V18TaskRecord) { task.NextCheckIn = "15/03/2026" },
		"negative repeat": func(task *format.V18TaskRecord) {
			task.NextCheckIn = "2026-03-15"
			task.CheckInRepeatDays = -3
		},
	} {
		t.Run(name, func(t *testing.T) {
			document := v19ProjectTestDocument(t)
			mutate(&document.Tasks[0])
			if _, err := WriteV19ProjectDocument(document); err == nil {
				t.Fatalf("the document writer accepted %s", name)
			}
		})
	}
}

// TestV18ProjectDocumentStaysBlindToCheckIns freezes the historical codec: a
// schema-18 reader that learned the newer keys would silently accept documents
// its own writer could never have produced, and the archives a migration
// validates against would stop being a fixed target.
func TestV18ProjectDocumentStaysBlindToCheckIns(t *testing.T) {
	document := v19ProjectTestDocument(t)
	document.Tasks[0].NextCheckIn = "2026-03-15"
	published, err := WriteV19ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV19ProjectDocument: %v", err)
	}
	downgraded := strings.Replace(published, schemaMarkerV19, schemaMarkerV18, 1)
	if _, err := ParseV18ProjectDocument(downgraded); err == nil {
		t.Fatalf("schema-18 reader accepted a check-in:\n%s", downgraded)
	}
}

// TestV19ProjectDocumentRefusesAnotherSchemasMarker is the ordinary strictness
// every document codec here owes: the marker names the grammar, so reading a
// neighboring schema's file as this one is refused rather than attempted.
func TestV19ProjectDocumentRefusesAnotherSchemasMarker(t *testing.T) {
	published, err := WriteV19ProjectDocument(v19ProjectTestDocument(t))
	if err != nil {
		t.Fatalf("WriteV19ProjectDocument: %v", err)
	}
	for _, marker := range []string{schemaMarkerV18, schemaMarkerV17} {
		if _, err := ParseV19ProjectDocument(strings.Replace(published, schemaMarkerV19, marker, 1)); err == nil {
			t.Fatalf("schema-19 reader accepted %s", marker)
		}
	}
}
