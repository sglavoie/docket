package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV19 admits the check-in schedule and review history a task
// record can now carry.
const SchemaVersionV19 = 19
const schemaMarkerV19 = "<!-- tt-schema: 19 -->"

// V19ProjectDocument is one project task file in schema 19: the schema-18
// document with schema-18 task records in place of schema-17 ones. The
// project-comment collection is unchanged and is carried, not re-derived, so a
// project's comments survive the boundary exactly as its task bodies do.
type V19ProjectDocument struct {
	ProjectName string
	Comments    []format.ProjectCommentRecord
	Tasks       []format.V18TaskRecord
}

// ValidateV19ProjectDocument validates the comment collection, every
// schema-18 task record, and then the project-wide invariants through the
// schema-18 document that owns them. The projection drops only what schema 19
// added, so a document this accepts is one schema 18 would have accepted before
// the three new keys existed.
func ValidateV19ProjectDocument(document V19ProjectDocument) error {
	if err := format.ValidateProjectCommentRecords(document.Comments); err != nil {
		return err
	}
	base := V18ProjectDocument{ProjectName: document.ProjectName, Comments: document.Comments}
	for _, task := range document.Tasks {
		if err := format.ValidateV18TaskRecord(task); err != nil {
			return err
		}
		base.Tasks = append(base.Tasks, task.V17TaskRecord)
	}
	return ValidateV18ProjectDocument(base)
}

// ParseV19ProjectDocument strictly parses schema-19 project Markdown.
func ParseV19ProjectDocument(content string) (V19ProjectDocument, error) {
	document, err := scanV19ProjectDocument(content)
	if err != nil {
		return V19ProjectDocument{}, err
	}
	return document, ValidateV19ProjectDocument(document)
}

// scanV19ProjectDocument reads schema-19 Markdown into records without the
// document-wide validation, so a repair reader can judge that validation itself.
func scanV19ProjectDocument(content string) (V19ProjectDocument, error) {
	scanned, err := scanTaskProjectDocumentWithComments(content, SchemaVersionV19, "v19")
	if err != nil {
		return V19ProjectDocument{}, err
	}
	document := V19ProjectDocument{ProjectName: scanned.ProjectName, Comments: scanned.Comments}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV18TaskBody(block.Title, block.Body)
		if err != nil {
			return V19ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	return document, nil
}

// WriteV19ProjectDocument renders canonical schema-19 Markdown.
func WriteV19ProjectDocument(document V19ProjectDocument) (string, error) {
	if err := ValidateV19ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	body, err := writeTaskProjectDocument(document.ProjectName, schemaMarkerV19, records, func(i int) (string, error) {
		return format.WriteV18TaskBody(document.Tasks[i])
	})
	if err != nil {
		return "", err
	}
	comments, err := format.WriteProjectCommentsBlock(document.Comments)
	if err != nil || comments == "" {
		return body, err
	}
	marker := schemaMarkerV19 + "\n"
	index := strings.Index(body, marker)
	if index < 0 {
		return "", fmt.Errorf("v19 project document is missing its schema marker")
	}
	cut := index + len(marker)
	return body[:cut] + comments + body[cut:], nil
}

// V19ProjectDocumentFromV18 lifts a schema-18 document without changing any
// existing value. No retired field carried a check-in schedule or a review
// instant into this boundary, so every migrated task starts with all three
// absent, which is exactly the three omitted metadata keys: the lifted document
// renders byte-for-byte what its source rendered, apart from the marker line.
func V19ProjectDocumentFromV18(source V18ProjectDocument) V19ProjectDocument {
	document := V19ProjectDocument{ProjectName: source.ProjectName, Comments: source.Comments}
	for _, task := range source.Tasks {
		document.Tasks = append(document.Tasks, format.V18TaskRecord{V17TaskRecord: task})
	}
	return document
}

// SeedWaitingCheckInsV19 gives every Waiting task that already carries a
// deadline a one-off check-in on that same calendar day, and changes nothing
// else. It is the schema-19 boundary's one content edit, separate from the lift
// above so the lift stays provably value-preserving and this stays the one place
// a migrated value is minted.
//
// Waiting is the state that means "somebody else owes me something", so its
// deadline is the day the wait is meant to be looked at again; copying it is a
// reading of what the task already said rather than a new commitment. The copy
// is a string copy of the canonical YYYY-MM-DD deadline, which is why the
// deadline, its optional companion clock, both timestamps, the body, the
// comments, and every other stored value are untouched: the deadline is never
// read, converted, or rewritten, only duplicated into a key that was absent.
//
// The repeat stays off and no review instant is invented. A cadence and a
// history are claims about how this task is worked and when it was last
// answered; the migration knows neither, and a review instant it made up would
// be indistinguishable afterwards from one a person earned.
//
// A task that already carries a check-in keeps it. Nothing at this boundary can
// produce one, but a re-run over an already-seeded document has to be the
// identity, which is what lets the migration be repeatable.
func SeedWaitingCheckInsV19(document V19ProjectDocument) V19ProjectDocument {
	seeded := V19ProjectDocument{ProjectName: document.ProjectName, Comments: document.Comments}
	seeded.Tasks = make([]format.V18TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		if task.State == string(models.StateWaiting) && task.NextCheckIn == "" && task.DueDate != "" {
			task.NextCheckIn = task.DueDate
		}
		seeded.Tasks = append(seeded.Tasks, task)
	}
	return seeded
}
