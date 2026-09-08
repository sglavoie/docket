package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// This file owns the live document lifecycle alone: parse, load, canonicalize,
// render, and hash. It names no schema version, because two files beside it
// answer that: current_task_schema.go selects the codec, and
// current_task_conversion.go crosses between records and live models.

// ProjectDocument is the strict live storage representation of tasks.md.
type ProjectDocument struct {
	ProjectName   string
	Tasks         []*models.Task
	SchemaVersion int
	Revision      string
}

func ParseProjectDocument(content string) (*ProjectDocument, error) {
	parsed, err := parseCurrentTaskProjectDocument(strings.TrimPrefix(content, "\uFEFF"))
	if err != nil {
		return nil, err
	}
	tasks, err := modelsFromCurrentTaskDocument(parsed, time.Local)
	if err != nil {
		return nil, err
	}
	doc := &ProjectDocument{ProjectName: parsed.ProjectName, Tasks: tasks, SchemaVersion: CurrentTaskSchemaVersion}
	doc.Revision = doc.ComputeRevision()
	return doc, nil
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
	document, err := currentTaskProjectDocumentFromModel(&models.Project{Name: d.ProjectName, Tasks: d.Tasks})
	if err != nil {
		return "", err
	}
	return writeCurrentTaskProjectDocument(document)
}

func ToProjectDocument(project *models.Project) *ProjectDocument {
	doc := &ProjectDocument{ProjectName: project.Name, Tasks: project.Tasks, SchemaVersion: CurrentTaskSchemaVersion}
	doc.Revision = doc.ComputeRevision()
	return doc
}

func ProjectDocumentFromBase(project *models.Project, _ *ProjectDocument) *ProjectDocument {
	return ToProjectDocument(project)
}

func CanonicalProjectDocument(project *models.Project, base *ProjectDocument) (*ProjectDocument, error) {
	doc := ProjectDocumentFromBase(project, base)
	content, err := doc.WriteCurrent()
	if err != nil {
		return nil, fmt.Errorf("rendering canonical project document: %w", err)
	}
	canonical, err := ParseProjectDocument(content)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing project document: %w", err)
	}
	return canonical, nil
}

func LoadProjectDocument(project *models.Project) (*ProjectDocument, error) {
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		doc := &ProjectDocument{ProjectName: project.Name, Tasks: []*models.Task{}, SchemaVersion: CurrentTaskSchemaVersion}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}
