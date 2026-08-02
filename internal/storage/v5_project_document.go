package storage

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// SchemaVersionV5 is the strict live task Markdown schema.
const SchemaVersionV5 = 5

const scannerInitBufSize = 64 * 1024

var (
	v5TaskHeadingRegex = regexp.MustCompile(`^(#{2,6})\s+Task:\s*(.*)$`)
	headingRegex       = regexp.MustCompile(`^(#{2,6})\s+Task:\s*(.*)$`)
	projectRegex       = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)
	schemaMarkerRegex  = regexp.MustCompile(`^<!--\s*tt-schema:\s*(\d+)\s*-->\s*$`)
)

// V5ProjectDocument is the target project task file. Tags are deliberately not
// stored at project level; callers derive them from Tasks.
type V5ProjectDocument struct {
	ProjectName string
	Tasks       []format.V5TaskRecord
}

// ValidateV5ProjectDocument validates target task records, project-local
// parent references, and identity uniqueness without touching live runtime
// models or tag catalogs.
func ValidateV5ProjectDocument(document V5ProjectDocument) error {
	if strings.TrimSpace(document.ProjectName) == "" ||
		document.ProjectName != strings.TrimSpace(document.ProjectName) {
		return fmt.Errorf("v5 project name is required and must be trimmed")
	}
	if len(document.ProjectName) > models.MaxProjectNameLength {
		return fmt.Errorf("v5 project name exceeds %d characters", models.MaxProjectNameLength)
	}
	if strings.ContainsAny(document.ProjectName, "\n\r") {
		return fmt.Errorf("v5 project name cannot contain newlines")
	}

	byID := make(map[string]format.V5TaskRecord, len(document.Tasks))
	shortIDs := make(map[int]bool, len(document.Tasks))
	for _, task := range document.Tasks {
		if err := format.ValidateV5TaskRecord(task); err != nil {
			return err
		}
		if _, duplicate := byID[task.ID]; duplicate {
			return fmt.Errorf("v5 project %q has duplicate task id %q", document.ProjectName, task.ID)
		}
		if shortIDs[task.ShortID] {
			return fmt.Errorf("v5 project %q has duplicate short id %d", document.ProjectName, task.ShortID)
		}
		byID[task.ID] = task
		shortIDs[task.ShortID] = true
	}
	for _, task := range document.Tasks {
		if task.ParentID != "" {
			if _, ok := byID[task.ParentID]; !ok {
				return fmt.Errorf("v5 task %q references missing project-local parent %q", task.ID, task.ParentID)
			}
		}
	}
	for _, task := range document.Tasks {
		seen := map[string]bool{task.ID: true}
		depth := 0
		for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
			if seen[parentID] {
				return fmt.Errorf("v5 project %q contains a parent cycle at task %q", document.ProjectName, task.ID)
			}
			seen[parentID] = true
			depth++
			if depth > models.MaxNestingDepth {
				return fmt.Errorf("v5 task %q is nested at depth %d, exceeding maximum depth %d", task.ID, depth, models.MaxNestingDepth)
			}
		}
	}
	return nil
}

// WriteV5ProjectDocument renders canonical schema-v5 Markdown. Every heading
// is the generic Task label, and no project tag catalog metadata is emitted.
func WriteV5ProjectDocument(document V5ProjectDocument) (string, error) {
	if err := ValidateV5ProjectDocument(document); err != nil {
		return "", err
	}
	byID := make(map[string]format.V5TaskRecord, len(document.Tasks))
	for _, task := range document.Tasks {
		byID[task.ID] = task
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "# Project: %s\n", document.ProjectName)
	fmt.Fprintf(&builder, "<!-- tt-schema: %d -->\n\n", SchemaVersionV5)
	for i, task := range document.Tasks {
		level := 2 + v5TaskDepth(task, byID)
		level = min(level, 6)
		fmt.Fprintf(&builder, "%s Task: %s\n", strings.Repeat("#", level), task.Title)
		body, err := format.WriteV5TaskBody(task)
		if err != nil {
			return "", err
		}
		builder.WriteString(body)
		if i < len(document.Tasks)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

func v5TaskDepth(task format.V5TaskRecord, byID map[string]format.V5TaskRecord) int {
	depth := 0
	for parentID := task.ParentID; parentID != ""; parentID = byID[parentID].ParentID {
		depth++
	}
	return depth
}

// ParseV5ProjectDocument strictly parses target schema-v5 Markdown. Legacy
// typed headings, legacy metadata, project tag catalogs, and other schema
// versions are rejected rather than normalized by the live v2 parser.
func ParseV5ProjectDocument(content string) (V5ProjectDocument, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, scannerInitBufSize), 16*1024*1024)

	var document V5ProjectDocument
	var schemaSeen bool
	var hasCurrent bool
	var currentTitle string
	var currentBody []string

	flush := func() error {
		if !hasCurrent {
			return nil
		}
		task, err := format.ParseV5TaskBody(currentTitle, strings.Join(currentBody, "\n"))
		if err != nil {
			return err
		}
		document.Tasks = append(document.Tasks, task)
		hasCurrent = false
		currentTitle = ""
		currentBody = nil
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if match := v5TaskHeadingRegex.FindStringSubmatch(line); match != nil {
			if !schemaSeen || document.ProjectName == "" {
				return V5ProjectDocument{}, fmt.Errorf("v5 task heading appears before project header")
			}
			if err := flush(); err != nil {
				return V5ProjectDocument{}, err
			}
			hasCurrent = true
			currentTitle = strings.TrimSpace(match[2])
			currentBody = nil
			continue
		}
		if headingRegex.MatchString(line) {
			return V5ProjectDocument{}, fmt.Errorf("v5 task document accepts only generic Task headings: %q", line)
		}
		if hasCurrent {
			currentBody = append(currentBody, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if match := projectRegex.FindStringSubmatch(line); match != nil {
			if document.ProjectName != "" {
				return V5ProjectDocument{}, fmt.Errorf("duplicate v5 project heading")
			}
			document.ProjectName = strings.TrimSpace(match[1])
			continue
		}
		if match := schemaMarkerRegex.FindStringSubmatch(line); match != nil {
			if schemaSeen {
				return V5ProjectDocument{}, fmt.Errorf("duplicate v5 schema marker")
			}
			if match[1] != "5" {
				return V5ProjectDocument{}, fmt.Errorf("unsupported v5 task schema %s", match[1])
			}
			schemaSeen = true
			continue
		}
		return V5ProjectDocument{}, fmt.Errorf("unexpected v5 project header line %q", line)
	}
	if err := scanner.Err(); err != nil {
		return V5ProjectDocument{}, fmt.Errorf("scan v5 task document: %w", err)
	}
	if err := flush(); err != nil {
		return V5ProjectDocument{}, err
	}
	if document.ProjectName == "" {
		return V5ProjectDocument{}, fmt.Errorf("v5 task document is missing project name")
	}
	if !schemaSeen {
		return V5ProjectDocument{}, fmt.Errorf("v5 task document is missing schema marker")
	}
	if err := ValidateV5ProjectDocument(document); err != nil {
		return V5ProjectDocument{}, err
	}
	return document, nil
}

// V5ProjectDocumentFromModel adapts the live lean task model to the strict v5
// storage boundary. The conversion is lossless for every retained runtime
// field and has no representation for retired task metadata.
func V5ProjectDocumentFromModel(project *models.Project) (V5ProjectDocument, error) {
	if project == nil {
		return V5ProjectDocument{}, fmt.Errorf("v5 project model is required")
	}
	document := V5ProjectDocument{
		ProjectName: project.Name,
		Tasks:       make([]format.V5TaskRecord, 0, len(project.Tasks)),
	}
	for _, task := range project.Tasks {
		record, err := v5TaskRecordFromModel(task)
		if err != nil {
			return V5ProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, record)
	}
	if err := ValidateV5ProjectDocument(document); err != nil {
		return V5ProjectDocument{}, err
	}
	return document, nil
}

// ModelsFromV5ProjectDocument reconstructs live tasks from a strict v5
// document. Date-only values are interpreted in the supplied workspace-local
// timezone.
func ModelsFromV5ProjectDocument(document V5ProjectDocument, location *time.Location) ([]*models.Task, error) {
	if location == nil {
		return nil, fmt.Errorf("v5 task model conversion requires a local timezone")
	}
	if err := ValidateV5ProjectDocument(document); err != nil {
		return nil, err
	}
	tasks := make([]*models.Task, 0, len(document.Tasks))
	for _, record := range document.Tasks {
		task, err := modelTaskFromV5Record(record, location)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func v5TaskRecordFromModel(task *models.Task) (format.V5TaskRecord, error) {
	if task == nil {
		return format.V5TaskRecord{}, fmt.Errorf("v5 project contains a nil task")
	}
	createdAt := task.CreatedAt
	if createdAt.IsZero() {
		// Programmatic repository adapters predating v5 can still construct lean
		// tasks directly. Persist a deterministic non-zero lifecycle baseline
		// rather than emitting an invalid v5 record or inventing wall-clock time.
		createdAt = time.Unix(0, 0).UTC()
	}
	updatedAt := task.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	record := format.V5TaskRecord{
		ID:          task.ID,
		ShortID:     task.ShortID,
		Title:       task.Title,
		Description: task.Description,
		State:       task.State().String(),
		Priority:    task.Priority,
		Tags:        append([]string(nil), task.Tags...),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		CompletedAt: cloneV5Time(task.CompletedAt),
		Pinned:      task.Pinned,
		PinnedAt:    cloneV5Time(task.PinnedAt),
	}
	if task.ParentID != nil {
		record.ParentID = *task.ParentID
	}
	if task.DueDate != nil {
		record.DueDate = task.DueDate.Format("2006-01-02")
	}
	if task.SnoozedUntil != nil {
		record.SnoozedUntil = task.SnoozedUntil.Format("2006-01-02")
	}
	for _, attachment := range task.Attachments {
		kind := "file"
		switch {
		case attachment.IsCodeRef:
			kind = "code"
		case attachment.IsURL:
			kind = "url"
		}
		recordAttachment := format.V5TaskAttachment{
			Kind:         kind,
			Filename:     attachment.Filename,
			OriginalPath: attachment.OriginalPath,
			LineNumber:   attachment.LineNumber,
		}
		if !attachment.AddedAt.IsZero() {
			addedAt := attachment.AddedAt
			recordAttachment.AddedAt = &addedAt
		}
		record.Attachments = append(record.Attachments, recordAttachment)
	}
	for _, item := range task.Checklist {
		record.Checklist = append(record.Checklist, format.V5ChecklistItem{
			Text: item.Text,
			Done: item.Done,
		})
	}
	for _, comment := range task.Comments {
		if comment == nil {
			return format.V5TaskRecord{}, fmt.Errorf("v5 task %q contains a nil comment", task.ID)
		}
		createdAt := comment.CreatedAt
		if createdAt.IsZero() {
			createdAt = updatedAt
		}
		record.Comments = append(record.Comments, format.V5TaskComment{
			Text:      comment.Text,
			CreatedAt: createdAt,
		})
	}
	for _, decision := range task.Decisions {
		if decision == nil {
			return format.V5TaskRecord{}, fmt.Errorf("v5 task %q contains a nil decision", task.ID)
		}
		createdAt := decision.CreatedAt
		if createdAt.IsZero() {
			createdAt = updatedAt
		}
		record.Decisions = append(record.Decisions, format.V5TaskDecision{
			Text:      decision.Text,
			Rationale: decision.Rationale,
			CreatedAt: createdAt,
		})
	}
	if err := format.ValidateV5TaskRecord(record); err != nil {
		return format.V5TaskRecord{}, err
	}
	return record, nil
}

func modelTaskFromV5Record(record format.V5TaskRecord, location *time.Location) (*models.Task, error) {
	state, ok := models.ParseWorkflowState(record.State)
	if !ok {
		return nil, fmt.Errorf("v5 task %q has invalid state %q", record.ID, record.State)
	}
	task := &models.Task{
		ID:            record.ID,
		ShortID:       record.ShortID,
		Title:         record.Title,
		Description:   record.Description,
		WorkflowState: state,
		Priority:      record.Priority,
		Tags:          append([]string{}, record.Tags...),
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
		CompletedAt:   cloneV5Time(record.CompletedAt),
		Pinned:        record.Pinned,
		PinnedAt:      cloneV5Time(record.PinnedAt),
		Comments:      []*models.Comment{},
		Decisions:     []*models.Decision{},
		Attachments:   []models.Attachment{},
		Checklist:     []models.ChecklistItem{},
		Children:      []*models.Task{},
	}
	if record.ParentID != "" {
		parentID := record.ParentID
		task.ParentID = &parentID
	}
	if record.DueDate != "" {
		due, err := time.ParseInLocation("2006-01-02", record.DueDate, location)
		if err != nil {
			return nil, fmt.Errorf("parse v5 task %q due date: %w", record.ID, err)
		}
		task.DueDate = &due
	}
	if record.SnoozedUntil != "" {
		snoozed, err := time.ParseInLocation("2006-01-02", record.SnoozedUntil, location)
		if err != nil {
			return nil, fmt.Errorf("parse v5 task %q snooze date: %w", record.ID, err)
		}
		task.SnoozedUntil = &snoozed
	}
	for _, attachment := range record.Attachments {
		modelAttachment := models.Attachment{
			Filename:     attachment.Filename,
			OriginalPath: attachment.OriginalPath,
			LineNumber:   attachment.LineNumber,
			IsURL:        attachment.Kind == "url",
			IsCodeRef:    attachment.Kind == "code",
		}
		if attachment.AddedAt != nil {
			modelAttachment.AddedAt = *attachment.AddedAt
		}
		task.Attachments = append(task.Attachments, modelAttachment)
	}
	for _, item := range record.Checklist {
		task.Checklist = append(task.Checklist, models.ChecklistItem{Text: item.Text, Done: item.Done})
	}
	for _, comment := range record.Comments {
		task.Comments = append(task.Comments, &models.Comment{Text: comment.Text, CreatedAt: comment.CreatedAt})
	}
	for _, decision := range record.Decisions {
		task.Decisions = append(task.Decisions, &models.Decision{
			Text:      decision.Text,
			Rationale: decision.Rationale,
			CreatedAt: decision.CreatedAt,
		})
	}
	return task, nil
}

func cloneV5Time(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
