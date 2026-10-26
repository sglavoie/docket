package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// This file owns the live document lifecycle alone: parse, load, canonicalize,
// render, and hash. It names no schema version, because two files beside it
// answer that: current_task_schema.go selects the codec, and
// current_task_conversion.go crosses between records and live models.

// ProjectDocument is the strict live storage representation of tasks.md. It
// carries the project's own comment collection beside its tasks, because both
// live in that one file and a document that dropped either would rewrite it on
// the next save.
type ProjectDocument struct {
	ProjectName   string
	Tasks         []*models.Task
	Comments      []*models.ProjectComment
	SchemaVersion int
	Revision      string
}

func ParseProjectDocument(content string) (*ProjectDocument, error) {
	doc, err := parseProjectDocumentContent(content)
	if err != nil {
		return nil, err
	}
	doc.Revision = doc.ComputeRevision()
	return doc, nil
}

// parseProjectDocumentContent is ParseProjectDocument without the revision. The
// revision re-renders and re-validates the whole document, about half the cost
// of a parse, so a caller that keeps only the tasks and comments skips it.
func parseProjectDocumentContent(content string) (*ProjectDocument, error) {
	parsed, err := parseCurrentTaskProjectDocument(strings.TrimPrefix(content, "\uFEFF"))
	if err != nil {
		return nil, err
	}
	tasks, err := modelsFromCurrentTaskDocument(parsed, time.Local)
	if err != nil {
		return nil, err
	}
	comments, err := projectCommentsFromCurrentDocument(parsed)
	if err != nil {
		return nil, err
	}
	return &ProjectDocument{
		ProjectName: parsed.ProjectName, Tasks: tasks, Comments: comments,
		SchemaVersion: CurrentTaskSchemaVersion,
	}, nil
}

// ComputeRevision hashes the canonical rendering rather than the bytes a caller
// happens to hold, so two documents that mean the same thing hash the same and
// a whitespace-only file edit cannot expire a revision.
func (d *ProjectDocument) ComputeRevision() string {
	sum := sha256.Sum256([]byte(d.canonicalContent()))
	return hex.EncodeToString(sum[:])
}

func (d *ProjectDocument) canonicalContent() string {
	content, err := d.render()
	if err != nil {
		return ""
	}
	return content
}

func (d *ProjectDocument) WriteCurrent() (string, error) {
	if d.SchemaVersion != CurrentTaskSchemaVersion {
		return "", fmt.Errorf("unsupported project schema %d", d.SchemaVersion)
	}
	return d.render()
}

func (d *ProjectDocument) render() (string, error) {
	return WriteProjectMarkdown(&models.Project{Name: d.ProjectName, Tasks: d.Tasks, Comments: d.Comments})
}

// ToProjectDocument projects a live project into a document. The task pointers
// are shared with the project on purpose — a mutation edits them in place — but
// the comment collection is detached, because a document is also the immutable
// base a merge and a rollback are measured against and nothing may edit one
// through the other.
func ToProjectDocument(project *models.Project) *ProjectDocument {
	doc := &ProjectDocument{
		ProjectName: project.Name, Tasks: project.Tasks,
		Comments: models.CloneProjectComments(project.Comments), SchemaVersion: CurrentTaskSchemaVersion,
	}
	doc.Revision = doc.ComputeRevision()
	return doc
}

// CanonicalProjectDocument is the project as the next load of its rendering
// will see it: rendered, parsed back, and revised.
func CanonicalProjectDocument(project *models.Project, _ *ProjectDocument) (*ProjectDocument, error) {
	canonical, _, err := canonicalProjectRendering(project)
	return canonical, err
}

// canonicalProjectRendering is CanonicalProjectDocument plus the canonical
// bytes its revision hashes, which are what a commit writes. Rendering the
// project, parsing that back, and rendering the parse once more is the whole
// cost: the revision is the digest of that last rendering rather than of a
// render of its own, and the writer has no reason to render it again.
func canonicalProjectRendering(project *models.Project) (*ProjectDocument, string, error) {
	content, err := WriteProjectMarkdown(&models.Project{Name: project.Name, Tasks: project.Tasks, Comments: project.Comments})
	if err != nil {
		return nil, "", fmt.Errorf("rendering canonical project document: %w", err)
	}
	canonical, rendered, err := canonicalizeProjectContent(content)
	if err != nil {
		return nil, "", fmt.Errorf("canonicalizing project document: %w", err)
	}
	return canonical, rendered, nil
}

// canonicalizeProjectContent parses rendered content and renders the parse,
// returning the document with the revision of that rendering and the rendering
// itself.
func canonicalizeProjectContent(content string) (*ProjectDocument, string, error) {
	canonical, err := parseProjectDocumentContent(content)
	if err != nil {
		return nil, "", err
	}
	rendered, err := canonical.render()
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256([]byte(rendered))
	canonical.Revision = hex.EncodeToString(sum[:])
	return canonical, rendered, nil
}

func LoadProjectDocument(project *models.Project) (*ProjectDocument, error) {
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		// An absent file means an empty project, comments included: initializing it
		// from the in-memory project would let a stale collection outlive the file
		// that was supposed to hold it.
		doc := &ProjectDocument{ProjectName: project.Name, Tasks: []*models.Task{}, SchemaVersion: CurrentTaskSchemaVersion}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}
