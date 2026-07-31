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

// ProjectDocument is the strict schema-v5 storage representation of tasks.md.
type ProjectDocument struct {
	ProjectName   string
	Tasks         []*models.Task
	SchemaVersion int
	Revision      string
}

func ParseProjectDocument(content string) (*ProjectDocument, error) {
	v5, err := ParseV5ProjectDocument(strings.TrimPrefix(content, "\uFEFF"))
	if err != nil {
		return nil, err
	}
	tasks, err := ModelsFromV5ProjectDocument(v5, time.Local)
	if err != nil {
		return nil, err
	}
	doc := &ProjectDocument{ProjectName: v5.ProjectName, Tasks: tasks, SchemaVersion: SchemaVersionV5}
	doc.Revision = doc.ComputeRevision()
	return doc, nil
}

func (d *ProjectDocument) ComputeRevision() string {
	sum := sha256.Sum256([]byte(d.canonicalContent()))
	return hex.EncodeToString(sum[:])
}

func (d *ProjectDocument) canonicalContent() string {
	content, err := d.writeV5()
	if err != nil {
		return ""
	}
	return content
}

func (d *ProjectDocument) WriteCurrent() (string, error) {
	if d.SchemaVersion != SchemaVersionV5 {
		return "", fmt.Errorf("unsupported project schema %d", d.SchemaVersion)
	}
	return d.writeV5()
}

func (d *ProjectDocument) writeV5() (string, error) {
	project := &models.Project{Name: d.ProjectName, Tasks: d.Tasks}
	v5, err := V5ProjectDocumentFromModel(project)
	if err != nil {
		return "", err
	}
	return WriteV5ProjectDocument(v5)
}

func ToProjectDocument(project *models.Project) *ProjectDocument {
	doc := &ProjectDocument{ProjectName: project.Name, Tasks: project.Tasks, SchemaVersion: SchemaVersionV5}
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
		doc := &ProjectDocument{ProjectName: project.Name, Tasks: []*models.Task{}, SchemaVersion: SchemaVersionV5}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}
