package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/storage/format"
)

// SchemaVersionV21 retires the someday lifecycle state: a schema-21 task record
// holds one of seven states, and the work schema 20 stored as someday is
// Backlog. The record grammar is otherwise schema 20's, unchanged.
const SchemaVersionV21 = 21
const schemaMarkerV21 = "<!-- tt-schema: 21 -->"

// retiredSomedayState is the state spelling schema 21 retires. It is spelled
// here rather than read from the models, which no longer name it, so this frozen
// codec keeps meaning what it meant when it was written.
const retiredSomedayState = "someday"

// V21ProjectDocument is one project task file in schema 21: the schema-20
// document under the seven-state vocabulary.
type V21ProjectDocument struct {
	ProjectName string
	Comments    []format.ProjectCommentRecord
	Tasks       []format.V19TaskRecord
}

// isV21State is schema 21's lifecycle vocabulary, frozen with the codec.
func isV21State(state string) bool {
	switch state {
	case "inbox", "backlog", "doing", "waiting", "paused", "done", "canceled":
		return true
	default:
		return false
	}
}

// ValidateV21TaskRecord validates one schema-21 record: the schema-19 record
// grammar, held to the seven states.
func ValidateV21TaskRecord(task format.V19TaskRecord) error {
	if !isV21State(task.State) {
		return fmt.Errorf("task %q has state %q, which schema 21 does not hold", task.ID, task.State)
	}
	return format.ValidateV19TaskRecord(task)
}

// ValidateV21ProjectDocument refuses a retired state on any record and then
// validates the rest through the schema-20 document, whose invariants are
// unchanged.
func ValidateV21ProjectDocument(document V21ProjectDocument) error {
	for _, task := range document.Tasks {
		if !isV21State(task.State) {
			return fmt.Errorf("task %q has state %q, which schema 21 does not hold", task.ID, task.State)
		}
	}
	return ValidateV20ProjectDocument(V20ProjectDocument(document))
}

// ParseV21ProjectDocument strictly parses schema-21 project Markdown.
func ParseV21ProjectDocument(content string) (V21ProjectDocument, error) {
	document, err := scanV21ProjectDocument(content)
	if err != nil {
		return V21ProjectDocument{}, err
	}
	return document, ValidateV21ProjectDocument(document)
}

// scanV21ProjectDocument reads schema-21 Markdown into records without the
// document-wide validation, so a repair reader can judge that validation itself.
func scanV21ProjectDocument(content string) (V21ProjectDocument, error) {
	scanned, err := scanTaskProjectDocumentWithComments(content, SchemaVersionV21, "v21")
	if err != nil {
		return V21ProjectDocument{}, err
	}
	document := V21ProjectDocument{ProjectName: scanned.ProjectName, Comments: scanned.Comments}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV19TaskBody(block.Title, block.Body)
		if err != nil {
			return V21ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	return document, nil
}

// WriteV21ProjectDocument renders canonical schema-21 Markdown.
func WriteV21ProjectDocument(document V21ProjectDocument) (string, error) {
	if err := ValidateV21ProjectDocument(document); err != nil {
		return "", err
	}
	return writeValidatedV21ProjectDocument(document)
}

// writeValidatedV21ProjectDocument renders a document the caller has just
// validated: schema 20's rendering under schema 21's marker.
func writeValidatedV21ProjectDocument(document V21ProjectDocument) (string, error) {
	body, err := writeValidatedV20ProjectDocument(V20ProjectDocument(document))
	if err != nil {
		return "", err
	}
	if !strings.Contains(body, schemaMarkerV20+"\n") {
		return "", fmt.Errorf("v21 project document is missing its schema marker")
	}
	return strings.Replace(body, schemaMarkerV20+"\n", schemaMarkerV21+"\n", 1), nil
}

// V21ProjectDocumentFromV20 lifts a schema-20 document and retires the someday
// state on the way: a task schema 20 stored as someday is Backlog in schema 21,
// which is the state the product folds it into. Nothing else moves — the state
// clock, the touch time, planning, and every other value are carried as they
// were, since retiring a vocabulary word is not a lifecycle transition anyone
// made. It returns how many records it retired.
func V21ProjectDocumentFromV20(source V20ProjectDocument) (V21ProjectDocument, int) {
	document := V21ProjectDocument{ProjectName: source.ProjectName, Comments: source.Comments}
	retired := 0
	for _, task := range source.Tasks {
		if task.State == retiredSomedayState {
			task.State = "backlog"
			retired++
		}
		document.Tasks = append(document.Tasks, task)
	}
	return document, retired
}
