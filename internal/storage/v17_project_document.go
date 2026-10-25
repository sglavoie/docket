package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/storage/format"
)

// SchemaVersionV17 adds the optional project-comment collection.
const SchemaVersionV17 = 17

// schemaMarkerV17 pins the exact marker line live task Markdown carries.
const schemaMarkerV17 = "<!-- tt-schema: 17 -->"

// V17ProjectDocument is one project task file in schema 17. The task records
// are schema 16's unchanged: this boundary advances the document, not the task
// grammar, so the record type is reused rather than lifted. What is new is the
// Comments collection, which belongs to the project and therefore rides above
// every task heading.
type V17ProjectDocument struct {
	ProjectName string
	Comments    []format.ProjectCommentRecord
	Tasks       []format.V16TaskRecord
}

// ValidateV17ProjectDocument validates the shared project invariants, the
// project-comment collection, and every schema-16 task record.
func ValidateV17ProjectDocument(document V17ProjectDocument) error {
	if err := format.ValidateProjectCommentRecords(document.Comments); err != nil {
		return err
	}
	return ValidateV16ProjectDocument(v16ProjectDocumentFromV17(document))
}

// v16ProjectDocumentFromV17 projects a schema-17 document back onto the schema
// it lifted from. The comment collection has no schema-16 spelling and is
// dropped rather than encoded, which is what makes this a projection and not a
// down-migration: it exists so the shared schema-16 validation reads exactly
// the document it owns.
func v16ProjectDocumentFromV17(document V17ProjectDocument) V16ProjectDocument {
	return V16ProjectDocument{ProjectName: document.ProjectName, Tasks: document.Tasks}
}

// ParseV17ProjectDocument strictly parses schema-17 project Markdown.
func ParseV17ProjectDocument(content string) (V17ProjectDocument, error) {
	scanned, err := scanTaskProjectDocumentWithComments(content, SchemaVersionV17, "v17")
	if err != nil {
		return V17ProjectDocument{}, err
	}
	document := V17ProjectDocument{ProjectName: scanned.ProjectName, Comments: scanned.Comments}
	for _, block := range scanned.Tasks {
		task, err := format.ParseV16TaskBody(block.Title, block.Body)
		if err != nil {
			return V17ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, task)
	}
	if err := ValidateV17ProjectDocument(document); err != nil {
		return V17ProjectDocument{}, err
	}
	return document, nil
}

// WriteV17ProjectDocument renders canonical schema-17 Markdown. The comment
// block is spliced between the header and the first task heading rather than
// threaded through writeTaskProjectDocument, because that helper is the frozen
// frame every historical schema renders through and this block exists in one
// schema only.
func WriteV17ProjectDocument(document V17ProjectDocument) (string, error) {
	if err := ValidateV17ProjectDocument(document); err != nil {
		return "", err
	}
	records := make([]format.V6TaskRecord, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		records = append(records, task.V6TaskRecord)
	}
	body, err := writeTaskProjectDocument(document.ProjectName, schemaMarkerV17, records, func(i int) (string, error) {
		return format.WriteV16TaskBody(document.Tasks[i])
	})
	if err != nil {
		return "", err
	}
	comments, err := format.WriteProjectCommentsBlock(document.Comments)
	if err != nil {
		return "", err
	}
	if comments == "" {
		return body, nil
	}
	return spliceProjectCommentsBlock(body, comments)
}

// spliceProjectCommentsBlock inserts the block directly below the schema marker
// line the shared frame just wrote, so it lands above the blank line that frame
// already puts before the first task heading. The block is therefore separated
// from the tasks below it and sits tight against the marker above it, which is
// exactly the shape the scanner reads back.
func spliceProjectCommentsBlock(body, comments string) (string, error) {
	marker := schemaMarkerV17 + "\n"
	index := strings.Index(body, marker)
	if index < 0 {
		return "", fmt.Errorf("v17 project document is missing its schema marker")
	}
	cut := index + len(marker)
	return body[:cut] + comments + body[cut:], nil
}

// V17ProjectDocumentFromV16 lifts a schema-16 document without changing any
// existing value. No retired field carried a project comment into this
// boundary, so every migrated project starts with an empty collection, which is
// exactly an omitted PROJECT-COMMENTS block. The task slice is carried by
// reference because its element type is unchanged.
func V17ProjectDocumentFromV16(document V16ProjectDocument) V17ProjectDocument {
	return V17ProjectDocument{ProjectName: document.ProjectName, Tasks: document.Tasks}
}
