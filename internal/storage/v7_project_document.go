package storage

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV7 adds the canonical lowercase-kebab tag contract while
// retaining schema 6's task fields and generic Markdown headings.
const SchemaVersionV7 = 7

type V7ProjectDocument = V6ProjectDocument

func ValidateV7ProjectDocument(document V7ProjectDocument) error {
	if err := ValidateV6ProjectDocument(document); err != nil {
		return err
	}
	for _, task := range document.Tasks {
		if err := format.ValidateV7TaskRecord(task); err != nil {
			return err
		}
	}
	return nil
}

func ParseV7ProjectDocument(content string) (V7ProjectDocument, error) {
	const current = "<!-- tt-schema: 7 -->"
	if strings.Count(content, current) != 1 {
		return V7ProjectDocument{}, fmt.Errorf("v7 task document requires exactly one schema marker")
	}
	legacyShape := strings.Replace(content, current, "<!-- tt-schema: 6 -->", 1)
	document, err := ParseV6ProjectDocument(legacyShape)
	if err != nil {
		return V7ProjectDocument{}, fmt.Errorf("parse v7 task document: %w", err)
	}
	if err := ValidateV7ProjectDocument(document); err != nil {
		return V7ProjectDocument{}, err
	}
	return document, nil
}

func WriteV7ProjectDocument(document V7ProjectDocument) (string, error) {
	if err := ValidateV7ProjectDocument(document); err != nil {
		return "", err
	}
	legacy, err := WriteV6ProjectDocument(document)
	if err != nil {
		return "", err
	}
	return strings.Replace(legacy, "<!-- tt-schema: 6 -->", "<!-- tt-schema: 7 -->", 1), nil
}
