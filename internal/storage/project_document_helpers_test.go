package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func mustWriteV5Project(t *testing.T, project *models.Project) string {
	t.Helper()
	document, err := CanonicalProjectDocument(project, nil)
	if err != nil {
		t.Fatal(err)
	}
	return mustWriteCurrentDocument(t, document)
}

func mustWriteCurrentDocument(t *testing.T, document *ProjectDocument) string {
	t.Helper()
	content, err := document.WriteCurrent()
	if err != nil {
		t.Fatal(err)
	}
	return content
}
