package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// These are the two current-schema setup helpers this package's tests share.
// Neither names a version: a test that wants specific historical bytes builds
// them from that schema's own codec, and a test that only needs a valid
// tasks.md to exist wants the live one.

// mustWriteCurrentProject canonicalizes a live project and renders it, which is
// what a test needs when it is staging a file the loader will read back.
func mustWriteCurrentProject(t *testing.T, project *models.Project) string {
	t.Helper()
	document, err := CanonicalProjectDocument(project, nil)
	if err != nil {
		t.Fatal(err)
	}
	return mustWriteCurrentDocument(t, document)
}

// mustWriteCurrentDocument renders a document a test already holds.
func mustWriteCurrentDocument(t *testing.T, document *ProjectDocument) string {
	t.Helper()
	content, err := document.WriteCurrent()
	if err != nil {
		t.Fatal(err)
	}
	return content
}
