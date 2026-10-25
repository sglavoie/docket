package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/storage/format"
)

// SchemaVersionV20 admits the stored hidden flag a task record can now carry.
const SchemaVersionV20 = 20
const schemaMarkerV20 = "<!-- tt-schema: 20 -->"

// V20ProjectDocument is one project task file in schema 20: the schema-19
// document with schema-19 task records in place of schema-18 ones. The
// project-comment collection is unchanged and is carried, not re-derived.
type V20ProjectDocument struct {
	ProjectName string
	Comments    []format.ProjectCommentRecord
	Tasks       []format.V19TaskRecord
}

// ValidateV20ProjectDocument validates every schema-19 task record and then the
// project-wide invariants through the schema-19 document that owns them. The
// projection drops only the hidden flag, so a document this accepts is one
// schema 19 would have accepted before the key existed.
func ValidateV20ProjectDocument(document V20ProjectDocument) error {
	base := V19ProjectDocument{ProjectName: document.ProjectName, Comments: document.Comments}
	for _, task := range document.Tasks {
		if err := format.ValidateV19TaskRecord(task); err != nil {
			return err
		}
		base.Tasks = append(base.Tasks, task.V18TaskRecord)
	}
	return ValidateV19ProjectDocument(base)
}

// ParseV20ProjectDocument strictly parses schema-20 project Markdown.
func ParseV20ProjectDocument(content string) (V20ProjectDocument, error) {
	document, err := scanV20ProjectDocument(content)
	if err != nil {
		return V20ProjectDocument{}, err
	}
	return document, ValidateV20ProjectDocument(document)
}

// scanV20ProjectDocument reads schema-20 Markdown into records without the
// document-wide validation, so a repair reader can judge that validation itself.
func scanV20ProjectDocument(content string) (V20ProjectDocument, error) {
	scanned, err := scanTaskProjectDocumentWithComments(content, SchemaVersionV20, "v20")
	if err != nil {
		return V20ProjectDocument{}, err
	}
	document := V20ProjectDocument{ProjectName: scanned.ProjectName, Comments: scanned.Comments}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV19TaskBody(block.Title, block.Body)
		if err != nil {
			return V20ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	return document, nil
}

// WriteV20ProjectDocument renders canonical schema-20 Markdown.
func WriteV20ProjectDocument(document V20ProjectDocument) (string, error) {
	if err := ValidateV20ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	body, err := writeTaskProjectDocument(document.ProjectName, schemaMarkerV20, records, func(i int) (string, error) {
		return format.WriteV19TaskBody(document.Tasks[i])
	})
	if err != nil {
		return "", err
	}
	comments, err := format.WriteProjectCommentsBlock(document.Comments)
	if err != nil || comments == "" {
		return body, err
	}
	marker := schemaMarkerV20 + "\n"
	index := strings.Index(body, marker)
	if index < 0 {
		return "", fmt.Errorf("v20 project document is missing its schema marker")
	}
	cut := index + len(marker)
	return body[:cut] + comments + body[cut:], nil
}

// V20ProjectDocumentFromV19 lifts a schema-19 document without changing any
// existing value. Nothing before this boundary could hide a task, so every
// migrated task starts unhidden, which is exactly the omitted metadata key: the
// lifted document renders byte-for-byte what its source rendered, apart from
// the marker line.
func V20ProjectDocumentFromV19(source V19ProjectDocument) V20ProjectDocument {
	document := V20ProjectDocument{ProjectName: source.ProjectName, Comments: source.Comments}
	for _, task := range source.Tasks {
		document.Tasks = append(document.Tasks, format.V19TaskRecord{V18TaskRecord: task})
	}
	return document
}
