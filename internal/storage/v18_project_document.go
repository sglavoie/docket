package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/storage/format"
)

// SchemaVersionV18 adds a checklist-origin record to promoted tasks.
const SchemaVersionV18 = 18
const schemaMarkerV18 = "<!-- tt-schema: 18 -->"

type V18ProjectDocument struct {
	ProjectName string
	Comments    []format.ProjectCommentRecord
	Tasks       []format.V17TaskRecord
}

func ValidateV18ProjectDocument(document V18ProjectDocument) error {
	if err := format.ValidateProjectCommentRecords(document.Comments); err != nil {
		return err
	}
	base := V17ProjectDocument{ProjectName: document.ProjectName, Comments: document.Comments}
	for _, task := range document.Tasks {
		if err := format.ValidateV17TaskRecord(task); err != nil {
			return err
		}
		base.Tasks = append(base.Tasks, task.V16TaskRecord)
	}
	return ValidateV17ProjectDocument(base)
}

func ParseV18ProjectDocument(content string) (V18ProjectDocument, error) {
	scanned, err := scanTaskProjectDocumentWithComments(content, SchemaVersionV18, "v18")
	if err != nil {
		return V18ProjectDocument{}, err
	}
	doc := V18ProjectDocument{ProjectName: scanned.ProjectName, Comments: scanned.Comments}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV17TaskBody(block.Title, block.Body)
		if err != nil {
			return V18ProjectDocument{}, err
		}
		doc.Tasks = append(doc.Tasks, task)
	}
	return doc, ValidateV18ProjectDocument(doc)
}

func WriteV18ProjectDocument(doc V18ProjectDocument) (string, error) {
	if err := ValidateV18ProjectDocument(doc); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(doc.Tasks))
	for _, task := range doc.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	body, err := writeTaskProjectDocument(doc.ProjectName, schemaMarkerV18, records, func(i int) (string, error) {
		return format.WriteV17TaskBody(doc.Tasks[i])
	})
	if err != nil {
		return "", err
	}
	comments, err := format.WriteProjectCommentsBlock(doc.Comments)
	if err != nil || comments == "" {
		return body, err
	}
	marker := schemaMarkerV18 + "\n"
	index := strings.Index(body, marker)
	if index < 0 {
		return "", fmt.Errorf("v18 project document is missing its schema marker")
	}
	cut := index + len(marker)
	return body[:cut] + comments + body[cut:], nil
}

func V18ProjectDocumentFromV17(source V17ProjectDocument) V18ProjectDocument {
	doc := V18ProjectDocument{ProjectName: source.ProjectName, Comments: source.Comments}
	for _, task := range source.Tasks {
		doc.Tasks = append(doc.Tasks, format.V17TaskRecord{V16TaskRecord: task})
	}
	return doc
}
