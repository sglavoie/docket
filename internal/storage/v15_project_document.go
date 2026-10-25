package storage

import "github.com/sglavoie/docket/internal/storage/format"

// SchemaVersionV15 lifts review off the tag list and onto a stored flag.
const SchemaVersionV15 = 15

// schemaMarkerV15 pins the exact marker line live task Markdown carries.
const schemaMarkerV15 = "<!-- tt-schema: 15 -->"

// V15ProjectDocument is one project task file in schema 15.
type V15ProjectDocument struct {
	ProjectName string
	Tasks       []format.V15TaskRecord
}

// ValidateV15ProjectDocument validates shared project invariants plus every
// schema-15 task record.
func ValidateV15ProjectDocument(document V15ProjectDocument) error {
	projected := v14ProjectDocumentFromV15(document)
	return validateTaskProjectDocument(projected.ProjectName, v6RecordsFromV14(projected.Tasks), func(i int) error {
		return format.ValidateV15TaskRecord(document.Tasks[i])
	})
}

// ParseV15ProjectDocument strictly parses schema-15 project Markdown.
func ParseV15ProjectDocument(content string) (V15ProjectDocument, error) {
	scanned, err := scanTaskProjectDocument(content, SchemaVersionV15, "v15")
	if err != nil {
		return V15ProjectDocument{}, err
	}
	document := V15ProjectDocument{ProjectName: scanned.ProjectName}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV15TaskBody(block.Title, block.Body)
		if err != nil {
			return V15ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV15ProjectDocument(document); err != nil {
		return V15ProjectDocument{}, err
	}
	return document, nil
}

// WriteV15ProjectDocument renders canonical schema-15 Markdown.
func WriteV15ProjectDocument(document V15ProjectDocument) (string, error) {
	if err := ValidateV15ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	return writeTaskProjectDocument(document.ProjectName, schemaMarkerV15, records, func(i int) (string, error) {
		return format.WriteV15TaskBody(document.Tasks[i])
	})
}

// V15ProjectDocumentFromV14 lifts a schema-14 document, moving the review tag
// onto the stored flag. The tag is removed as it is lifted: leaving both would
// make the workspace carry two answers to one question, and the next edit that
// rewrote the tag list would silently disagree with the lane.
//
// The tag name is matched exactly as it is persisted. Task tags are normalized
// on the way in, so a stored list holds one lowercase spelling and there is no
// case folding to do here.
func V15ProjectDocumentFromV14(document V14ProjectDocument) V15ProjectDocument {
	lifted := V15ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		lifted.Tasks = make([]format.V15TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		record := format.V15TaskRecord{V14TaskRecord: task}
		remaining := make([]string, 0, len(task.Tags))
		for _, tag := range task.Tags {
			if tag == LiftedReviewTaskTag {
				record.Review = true
				continue
			}
			remaining = append(remaining, tag)
		}
		if record.Review {
			record.Tags = remaining
		}
		lifted.Tasks = append(lifted.Tasks, record)
	}
	return lifted
}

// LiftedReviewTaskTag is the tag schema 15 lifts onto the stored review flag. It
// is named here rather than in a caller because the migration and any audit of
// it must agree on exactly one spelling.
const LiftedReviewTaskTag = "review"

func v14ProjectDocumentFromV15(document V15ProjectDocument) V14ProjectDocument {
	projected := V14ProjectDocument{ProjectName: document.ProjectName}
	if len(document.Tasks) > 0 {
		projected.Tasks = make([]format.V14TaskRecord, 0, len(document.Tasks))
	}
	for _, task := range document.Tasks {
		projected.Tasks = append(projected.Tasks, task.V14TaskRecord)
	}
	return projected
}

func v6RecordsFromV14(tasks []format.V14TaskRecord) []format.V6TaskRecord {
	records := make([]format.V6TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		records = append(records, task.V6TaskRecord)
	}
	return records
}
