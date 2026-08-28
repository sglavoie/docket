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

// ProjectDocument is the strict live schema-14 storage representation of tasks.md.
type ProjectDocument struct {
	ProjectName   string
	Tasks         []*models.Task
	SchemaVersion int
	Revision      string
}

func ParseProjectDocument(content string) (*ProjectDocument, error) {
	v14, err := ParseV14ProjectDocument(strings.TrimPrefix(content, "\uFEFF"))
	if err != nil {
		return nil, err
	}
	tasks, err := modelsFromV14ProjectDocument(v14, time.Local)
	if err != nil {
		return nil, err
	}
	doc := &ProjectDocument{ProjectName: v14.ProjectName, Tasks: tasks, SchemaVersion: SchemaVersionV14}
	doc.Revision = doc.ComputeRevision()
	return doc, nil
}

func (d *ProjectDocument) ComputeRevision() string {
	sum := sha256.Sum256([]byte(d.canonicalContent()))
	return hex.EncodeToString(sum[:])
}

func (d *ProjectDocument) canonicalContent() string {
	content, err := d.writeV14()
	if err != nil {
		return ""
	}
	return content
}

func (d *ProjectDocument) WriteCurrent() (string, error) {
	if d.SchemaVersion != SchemaVersionV14 {
		return "", fmt.Errorf("unsupported project schema %d", d.SchemaVersion)
	}
	return d.writeV14()
}

func (d *ProjectDocument) writeV14() (string, error) {
	v14, err := v14ProjectDocumentFromModel(&models.Project{Name: d.ProjectName, Tasks: d.Tasks})
	if err != nil {
		return "", err
	}
	return WriteV14ProjectDocument(v14)
}

func ToProjectDocument(project *models.Project) *ProjectDocument {
	doc := &ProjectDocument{ProjectName: project.Name, Tasks: project.Tasks, SchemaVersion: SchemaVersionV14}
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
		doc := &ProjectDocument{ProjectName: project.Name, Tasks: []*models.Task{}, SchemaVersion: SchemaVersionV14}
		doc.Revision = doc.ComputeRevision()
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	return ParseProjectDocument(string(content))
}

// modelsFromV14ProjectDocument builds live tasks from a schema-14 document.
// location resolves the bare due date only: the deferral value carries its own
// offset and is read as the instant it names. A due_time, when present, sets the
// due date's clock so models.Task.DueHasTime reports a timed due date.
func modelsFromV14ProjectDocument(document V14ProjectDocument, location *time.Location) ([]*models.Task, error) {
	if location == nil {
		location = time.Local
	}
	tasks := make([]*models.Task, 0, len(document.Tasks))
	for _, record := range document.Tasks {
		state, ok := models.ParseWorkflowState(record.State)
		if !ok {
			return nil, fmt.Errorf("v14 task %q has invalid live workflow state %q", record.ID, record.State)
		}
		task := &models.Task{
			ID: record.ID, ShortID: record.ShortID, Title: record.Title,
			Description: record.Description, WorkflowState: state, Priority: record.Priority,
			Tags: append([]string(nil), record.Tags...), Context: record.Context, DeferCount: record.DeferCount,
			CreatedAt: record.CreatedAt,
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
			task.DueDate, err = parseProjectDocumentDueDate(record.DueDate, record.DueTime, location)
			if err != nil {
				return nil, fmt.Errorf("v14 task %q due date: %w", record.ID, err)
			}
		}
		if record.SnoozedUntil != "" {
			task.DeferredUntil, err = parseProjectDocumentInstant(record.SnoozedUntil)
			if err != nil {
				return nil, fmt.Errorf("v14 task %q deferral instant: %w", record.ID, err)
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
		for _, dependency := range record.Dependencies {
			task.Dependencies = append(task.Dependencies, models.TaskReference{
				ProjectID: dependency.ProjectID, TaskID: dependency.TaskID,
			})
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func v14ProjectDocumentFromModel(project *models.Project) (V14ProjectDocument, error) {
	if project == nil {
		return V14ProjectDocument{}, fmt.Errorf("project is required")
	}
	document := V14ProjectDocument{ProjectName: project.Name}
	if len(project.Tasks) > 0 {
		document.Tasks = make([]format.V14TaskRecord, 0, len(project.Tasks))
	}
	for _, task := range project.Tasks {
		record, err := v14TaskRecordFromModel(task)
		if err != nil {
			return V14ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, record)
	}
	if err := ValidateV14ProjectDocument(document); err != nil {
		return V14ProjectDocument{}, err
	}
	return document, nil
}

func v14TaskRecordFromModel(task *models.Task) (format.V14TaskRecord, error) {
	if task == nil {
		return format.V14TaskRecord{}, fmt.Errorf("schema-14 writer cannot persist a nil task")
	}
	if !task.State().IsValid() {
		return format.V14TaskRecord{}, fmt.Errorf("schema-14 writer refuses invalid lifecycle %q on task %q", task.State(), task.ID)
	}
	if err := models.ValidatePriority(task.Priority, true); err != nil {
		return format.V14TaskRecord{}, fmt.Errorf("schema-14 writer refuses task %q priority: %w", task.ID, err)
	}
	if task.StateChangedAt.IsZero() {
		return format.V14TaskRecord{}, fmt.Errorf("schema-14 writer refuses task %q without a state clock", task.ID)
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
	dueTime := ""
	if task.DueDate != nil {
		record.DueDate = task.DueDate.Format("2006-01-02")
		if task.DueHasTime() {
			dueTime = task.DueDate.Format("15:04")
		}
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
	v14 := format.V14TaskRecord{
		V13TaskRecord: format.V13TaskRecord{
			V12TaskRecord: format.V12TaskRecord{
				V11TaskRecord: format.V11TaskRecord{
					V10TaskRecord: format.V10TaskRecord{V6TaskRecord: record, StateChangedAt: task.StateChangedAt},
					Context:       task.Context,
				},
				DeferCount: task.DeferCount,
			},
			DueTime: dueTime,
		},
	}
	for _, dependency := range task.Dependencies {
		v14.Dependencies = append(v14.Dependencies, format.V14TaskDependency{
			ProjectID: dependency.ProjectID, TaskID: dependency.TaskID,
		})
	}
	if err := format.ValidateV14TaskRecord(v14); err != nil {
		return format.V14TaskRecord{}, err
	}
	return v14, nil
}

func parseProjectDocumentDate(value string, location *time.Location) (*time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// parseProjectDocumentDueDate resolves the date-only "due" field, then folds in
// an optional "due_time" (HH:MM) so a timed due date carries its clock. A due
// date with no due_time stays at local midnight, which is the date-only signal.
func parseProjectDocumentDueDate(dateValue, timeValue string, location *time.Location) (*time.Time, error) {
	day, err := parseProjectDocumentDate(dateValue, location)
	if err != nil {
		return nil, err
	}
	if timeValue == "" {
		return day, nil
	}
	clock, err := time.ParseInLocation("15:04", timeValue, location)
	if err != nil {
		return nil, fmt.Errorf("due time %q: %w", timeValue, err)
	}
	due := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, location)
	return &due, nil
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
