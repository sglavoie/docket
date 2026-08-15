package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// ProjectDocument is the strict live schema-10 storage representation of tasks.md.
type ProjectDocument struct {
	ProjectName   string
	Tasks         []*models.Task
	SchemaVersion int
	Revision      string
}

func ParseProjectDocument(content string) (*ProjectDocument, error) {
	v10, err := ParseV10ProjectDocument(strings.TrimPrefix(content, "\uFEFF"))
	if err != nil {
		return nil, err
	}
	tasks, err := modelsFromV10ProjectDocument(v10, time.Local)
	if err != nil {
		return nil, err
	}
	doc := &ProjectDocument{ProjectName: v10.ProjectName, Tasks: tasks, SchemaVersion: SchemaVersionV10}
	doc.Revision = doc.ComputeRevision()
	return doc, nil
}

func (d *ProjectDocument) ComputeRevision() string {
	sum := sha256.Sum256([]byte(d.canonicalContent()))
	return hex.EncodeToString(sum[:])
}

func (d *ProjectDocument) canonicalContent() string {
	content, err := d.writeV10()
	if err != nil {
		return ""
	}
	return content
}

func (d *ProjectDocument) WriteCurrent() (string, error) {
	if d.SchemaVersion != SchemaVersionV10 {
		return "", fmt.Errorf("unsupported project schema %d", d.SchemaVersion)
	}
	return d.writeV10()
}

func (d *ProjectDocument) writeV10() (string, error) {
	v10, err := v10ProjectDocumentFromModel(&models.Project{Name: d.ProjectName, Tasks: d.Tasks})
	if err != nil {
		return "", err
	}
	return WriteV10ProjectDocument(v10)
}

func ToProjectDocument(project *models.Project) *ProjectDocument {
	doc := &ProjectDocument{ProjectName: project.Name, Tasks: project.Tasks, SchemaVersion: SchemaVersionV10}
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
		doc := &ProjectDocument{ProjectName: project.Name, Tasks: []*models.Task{}, SchemaVersion: SchemaVersionV10}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}

// modelsFromV10ProjectDocument builds live tasks from a schema-10 document.
// location resolves the bare due date only: the deferral value carries its own
// offset and is read as the instant it names.
func modelsFromV10ProjectDocument(document V10ProjectDocument, location *time.Location) ([]*models.Task, error) {
	if location == nil {
		location = time.Local
	}
	tasks := make([]*models.Task, 0, len(document.Tasks))
	for _, record := range document.Tasks {
		state, ok := models.ParseWorkflowState(record.State)
		if !ok {
			return nil, fmt.Errorf("v10 task %q has invalid live workflow state %q", record.ID, record.State)
		}
		task := &models.Task{
			ID: record.ID, ShortID: record.ShortID, Title: record.Title,
			Description: record.Description, WorkflowState: state, Priority: record.Priority,
			Tags: append([]string(nil), record.Tags...), CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt, StateChangedAt: record.StateChangedAt,
			CompletedAt: cloneProjectDocumentTime(record.CompletedAt),
			Pinned:      record.Pinned, PinnedAt: cloneProjectDocumentTime(record.PinnedAt),
			Children: []*models.Task{},
		}
		if record.ParentID != "" {
			parentID := record.ParentID
			task.ParentID = &parentID
		}
		var err error
		if record.DueDate != "" {
			task.DueDate, err = parseProjectDocumentDate(record.DueDate, location)
			if err != nil {
				return nil, fmt.Errorf("v10 task %q due date: %w", record.ID, err)
			}
		}
		if record.SnoozedUntil != "" {
			task.DeferredUntil, err = parseProjectDocumentInstant(record.SnoozedUntil)
			if err != nil {
				return nil, fmt.Errorf("v10 task %q deferral instant: %w", record.ID, err)
			}
		}
		for _, attachment := range record.Attachments {
			item := models.Attachment{
				Filename: attachment.Filename, OriginalPath: attachment.OriginalPath,
				LineNumber: attachment.LineNumber, IsURL: attachment.Kind == "url",
				IsCodeRef: attachment.Kind == "code",
			}
			if attachment.AddedAt != nil {
				item.AddedAt = *attachment.AddedAt
			}
			task.Attachments = append(task.Attachments, item)
		}
		for _, item := range record.Checklist {
			task.Checklist = append(task.Checklist, models.ChecklistItem{Text: item.Text, Done: item.Done})
		}
		for _, comment := range record.Comments {
			task.Comments = append(task.Comments, &models.Comment{Text: comment.Text, CreatedAt: comment.CreatedAt})
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func v10ProjectDocumentFromModel(project *models.Project) (V10ProjectDocument, error) {
	if project == nil {
		return V10ProjectDocument{}, fmt.Errorf("project is required")
	}
	document := V10ProjectDocument{ProjectName: project.Name}
	if len(project.Tasks) > 0 {
		document.Tasks = make([]format.V10TaskRecord, 0, len(project.Tasks))
	}
	for _, task := range project.Tasks {
		record, err := v10TaskRecordFromModel(task)
		if err != nil {
			return V10ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, record)
	}
	if err := ValidateV10ProjectDocument(document); err != nil {
		return V10ProjectDocument{}, err
	}
	return document, nil
}

func v10TaskRecordFromModel(task *models.Task) (format.V10TaskRecord, error) {
	if task == nil {
		return format.V10TaskRecord{}, fmt.Errorf("schema-10 writer cannot persist a nil task")
	}
	if !task.State().IsValid() {
		return format.V10TaskRecord{}, fmt.Errorf("schema-10 writer refuses invalid lifecycle %q on task %q", task.State(), task.ID)
	}
	if err := models.ValidatePriority(task.Priority, true); err != nil {
		return format.V10TaskRecord{}, fmt.Errorf("schema-10 writer refuses task %q priority: %w", task.ID, err)
	}
	if task.StateChangedAt.IsZero() {
		return format.V10TaskRecord{}, fmt.Errorf("schema-10 writer refuses task %q without a state clock", task.ID)
	}
	record := format.V6TaskRecord{
		ID: task.ID, ShortID: task.ShortID, Title: task.Title, Description: task.Description,
		State: task.State().String(), Priority: task.Priority, Tags: append([]string(nil), task.Tags...),
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		CompletedAt: cloneProjectDocumentTime(task.CompletedAt), Pinned: task.Pinned,
		PinnedAt: cloneProjectDocumentTime(task.PinnedAt),
	}
	if task.ParentID != nil {
		record.ParentID = *task.ParentID
	}
	if task.DueDate != nil {
		record.DueDate = task.DueDate.Format("2006-01-02")
	}
	if task.DeferredUntil != nil {
		record.SnoozedUntil = task.DeferredUntil.Format(time.RFC3339Nano)
	}
	for _, attachment := range task.Attachments {
		kind := "file"
		if attachment.IsURL {
			kind = "url"
		} else if attachment.IsCodeRef {
			kind = "code"
		}
		item := format.V6TaskAttachment{
			Kind: kind, Filename: attachment.Filename, OriginalPath: attachment.OriginalPath,
			LineNumber: attachment.LineNumber,
		}
		if !attachment.AddedAt.IsZero() {
			addedAt := attachment.AddedAt
			item.AddedAt = &addedAt
		}
		record.Attachments = append(record.Attachments, item)
	}
	for _, item := range task.Checklist {
		record.Checklist = append(record.Checklist, format.V6ChecklistItem{Text: item.Text, Done: item.Done})
	}
	for _, comment := range task.Comments {
		if comment == nil {
			continue
		}
		record.Comments = append(record.Comments, format.V6TaskComment{Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	v10 := format.V10TaskRecord{V6TaskRecord: record, StateChangedAt: task.StateChangedAt}
	if err := format.ValidateV10TaskRecord(v10); err != nil {
		return format.V10TaskRecord{}, err
	}
	return v10, nil
}

func parseProjectDocumentDate(value string, location *time.Location) (*time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// parseProjectDocumentInstant reads a durable deferral value. The stored string
// carries its own offset, so it takes no location: normalizing here would make
// ComputeRevision disagree across machines and churn the file on every read.
func parseProjectDocumentInstant(value string) (*time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func cloneProjectDocumentTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
