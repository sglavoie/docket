package storage

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/util"
)

// This file is the one crossing between the live models.Task graph and the
// current task-record schema. Every persisted task field is spelled here
// exactly twice, once reading and once writing, and nothing else in the live
// path builds a record or reads one back. A field that is not named in both
// halves is a field the workspace loses on the next save, so the two halves are
// kept adjacent rather than filed by concern.
//
// The schema those records belong to is selected in current_task_schema.go, so
// the helpers below name only "current". The one exception is the record
// literal in currentTaskRecordFromModel: a schema's record shape is its own
// nesting and cannot be written version-neutrally. That literal is the whole
// version-specific surface of the crossing.

// --- records to models ---------------------------------------------------

// modelsFromCurrentTaskDocument builds live tasks from a current-schema
// document. location resolves the bare due date only: the deferral value
// carries its own offset and is read as the instant it names. A due_time, when
// present, sets the due date's clock so models.Task.DueHasTime reports a timed
// due date.
func modelsFromCurrentTaskDocument(document currentTaskProjectDocument, location *time.Location) ([]*models.Task, error) {
	if location == nil {
		location = time.Local
	}
	tasks := make([]*models.Task, 0, len(document.Tasks))
	for _, record := range document.Tasks {
		task, err := taskFromCurrentTaskRecord(record, location)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// projectCommentsFromCurrentDocument reads the stored project-comment
// collection back into live values, in canonical order. Each element is a fresh
// pointer, so a project loaded twice from the same bytes never shares a comment
// object between the two answers.
func projectCommentsFromCurrentDocument(document currentTaskProjectDocument) ([]*models.ProjectComment, error) {
	if len(document.Comments) == 0 {
		return nil, nil
	}
	comments := make([]*models.ProjectComment, 0, len(document.Comments))
	for _, record := range document.Comments {
		if err := models.ValidateProjectComment(record.ID, record.Text, record.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, &models.ProjectComment{
			ID: record.ID, Text: record.Text, CreatedAt: record.CreatedAt,
		})
	}
	models.SortProjectComments(comments)
	return comments, nil
}

// taskFromCurrentTaskRecord reads one stored record back into the live model.
func taskFromCurrentTaskRecord(record currentTaskRecord, location *time.Location) (*models.Task, error) {
	state, ok := models.ParseWorkflowState(record.State)
	if !ok {
		return nil, fmt.Errorf("live task %q has invalid workflow state %q", record.ID, record.State)
	}
	task := &models.Task{
		ID: record.ID, ShortID: record.ShortID, Title: record.Title,
		Description: record.Description, WorkflowState: state, Priority: record.Priority,
		Tags: append([]string(nil), record.Tags...), Context: record.Context, DeferCount: record.DeferCount,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt, StateChangedAt: record.StateChangedAt,
		CompletedAt: cloneProjectDocumentTime(record.CompletedAt),
		Pinned:      record.Pinned, PinnedAt: cloneProjectDocumentTime(record.PinnedAt),
		Review:            record.Review,
		ResumeNote:        record.ResumeNote,
		ChecklistOrigin:   record.ChecklistOrigin,
		CheckInRepeatDays: record.CheckInRepeatDays,
		LastReviewedAt:    cloneProjectDocumentTime(record.LastReviewedAt),
		Children:          []*models.Task{},
	}
	if record.ParentID != "" {
		parentID := record.ParentID
		task.ParentID = &parentID
	}
	if err := applyCurrentTaskRecordSchedule(task, record, location); err != nil {
		return nil, err
	}
	appendCurrentTaskRecordCollections(task, record)
	return task, nil
}

// applyCurrentTaskRecordSchedule resolves the three stored time values that are
// spelled as text rather than as instants: the calendar due date with its
// optional clock, the deferral instant, and the calendar check-in date.
func applyCurrentTaskRecordSchedule(task *models.Task, record currentTaskRecord, location *time.Location) error {
	var err error
	if record.DueDate != "" {
		task.DueDate, err = parseProjectDocumentDueDate(record.DueDate, record.DueTime, location)
		if err != nil {
			return fmt.Errorf("live task %q due date: %w", record.ID, err)
		}
	}
	if record.SnoozedUntil != "" {
		task.DeferredUntil, err = parseProjectDocumentInstant(record.SnoozedUntil)
		if err != nil {
			return fmt.Errorf("live task %q deferral instant: %w", record.ID, err)
		}
	}
	if record.NextCheckIn != "" {
		task.NextCheckIn, err = parseProjectDocumentCheckInDate(record.NextCheckIn, location)
		if err != nil {
			return fmt.Errorf("live task %q check-in date: %w", record.ID, err)
		}
	}
	return nil
}

// appendCurrentTaskRecordCollections copies the repeated fields. Each is
// rebuilt element by element rather than assigned, so the live task never
// shares a backing array with the parsed record.
func appendCurrentTaskRecordCollections(task *models.Task, record currentTaskRecord) {
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
}

// --- models to records ---------------------------------------------------

// currentTaskProjectDocumentFromModel converts a live project into a validated
// current-schema document. It validates the whole document, not just each
// record, because the project-local invariants are what a single record cannot
// see.
func currentTaskProjectDocumentFromModel(project *models.Project) (currentTaskProjectDocument, error) {
	if project == nil {
		return currentTaskProjectDocument{}, fmt.Errorf("project is required")
	}
	document := currentTaskProjectDocument{ProjectName: project.Name}
	comments, commentsErr := currentProjectCommentRecordsFromModel(project.Comments)
	if commentsErr != nil {
		return currentTaskProjectDocument{}, commentsErr
	}
	document.Comments = comments
	if len(project.Tasks) > 0 {
		document.Tasks = make([]currentTaskRecord, 0, len(project.Tasks))
	}
	for _, task := range project.Tasks {
		record, err := currentTaskRecordFromModel(task)
		if err != nil {
			return currentTaskProjectDocument{}, err
		}
		document.Tasks = append(document.Tasks, record)
	}
	if err := validateCurrentTaskProjectDocument(document); err != nil {
		return currentTaskProjectDocument{}, err
	}
	return document, nil
}

// currentProjectCommentRecordsFromModel writes the live project-comment
// collection out as stored records, in canonical order. A collection the
// canonical writer would refuse is rejected here rather than rendered, so the
// refusal names the comment instead of the document.
func currentProjectCommentRecordsFromModel(comments []*models.ProjectComment) ([]currentProjectCommentRecord, error) {
	if len(comments) == 0 {
		return nil, nil
	}
	if err := models.ValidateProjectComments(comments); err != nil {
		return nil, fmt.Errorf("live project writer refuses project comment: %w", err)
	}
	ordered := models.CloneProjectComments(comments)
	models.SortProjectComments(ordered)
	records := make([]currentProjectCommentRecord, 0, len(ordered))
	for _, comment := range ordered {
		records = append(records, currentProjectCommentRecord{
			ID: comment.ID, Text: comment.Text, CreatedAt: comment.CreatedAt,
		})
	}
	return records, nil
}

// currentTaskRecordFromModel writes one live task out as a stored record.
func currentTaskRecordFromModel(task *models.Task) (currentTaskRecord, error) {
	if err := validateModelForCurrentTaskSchema(task); err != nil {
		return currentTaskRecord{}, err
	}
	base, dueTime := currentTaskBaseFieldsFromModel(task)
	appendCurrentTaskRecordCollectionsFromModel(&base, task)

	record := format.V18TaskRecord{V17TaskRecord: format.V17TaskRecord{V16TaskRecord: format.V16TaskRecord{
		V15TaskRecord: format.V15TaskRecord{
			V14TaskRecord: format.V14TaskRecord{
				V13TaskRecord: format.V13TaskRecord{
					V12TaskRecord: format.V12TaskRecord{
						V11TaskRecord: format.V11TaskRecord{
							V10TaskRecord: format.V10TaskRecord{V6TaskRecord: base, StateChangedAt: task.StateChangedAt},
							Context:       task.Context,
						},
						DeferCount: task.DeferCount,
					},
					DueTime: dueTime,
				},
			},
			Review: task.Review,
		},
		ResumeNote: task.ResumeNote,
	}, ChecklistOrigin: task.ChecklistOrigin,
	},
		NextCheckIn:       formatProjectDocumentCheckInDate(task.NextCheckIn),
		CheckInRepeatDays: task.CheckInRepeatDays,
		LastReviewedAt:    cloneProjectDocumentTime(task.LastReviewedAt),
	}
	for _, dependency := range task.Dependencies {
		record.Dependencies = append(record.Dependencies, format.V14TaskDependency{
			ProjectID: dependency.ProjectID, TaskID: dependency.TaskID,
		})
	}
	if err := validateCurrentTaskRecord(record); err != nil {
		return currentTaskRecord{}, err
	}
	return record, nil
}

// validateModelForCurrentTaskSchema refuses the live values the record codec
// cannot express. The state clock is one of them: a task without it is rejected
// here rather than backfilled, so a missing clock stays a bug in whoever
// dropped it.
func validateModelForCurrentTaskSchema(task *models.Task) error {
	if task == nil {
		return fmt.Errorf("live task writer cannot persist a nil task")
	}
	if !task.State().IsValid() {
		return fmt.Errorf("live task writer refuses invalid lifecycle %q on task %q", task.State(), task.ID)
	}
	if err := models.ValidatePriority(task.Priority, true); err != nil {
		return fmt.Errorf("live task writer refuses task %q priority: %w", task.ID, err)
	}
	if err := models.ValidateResumeNote(task.ResumeNote); err != nil {
		return fmt.Errorf("live task writer refuses task %q resume note: %w", task.ID, err)
	}
	if task.StateChangedAt.IsZero() {
		return fmt.Errorf("live task writer refuses task %q without a state clock", task.ID)
	}
	// The schedule is measured against the model's own gate rather than against a
	// second rule spelled here, so a check-in the editing surfaces refuse is a
	// check-in this writer refuses, named the same way.
	if err := models.ValidateCheckInSchedule(task.CheckIn()); err != nil {
		return fmt.Errorf("live task writer refuses task %q check-in: %w", task.ID, err)
	}
	if task.LastReviewedAt != nil && task.LastReviewedAt.IsZero() {
		return fmt.Errorf("live task writer refuses task %q review instant: a present value must be a real instant", task.ID)
	}
	return nil
}

// currentTaskBaseFieldsFromModel fills the layout fields every task schema
// shares, and returns the due clock separately because it is stored beside them
// rather than in them.
func currentTaskBaseFieldsFromModel(task *models.Task) (format.V6TaskRecord, string) {
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
	return record, dueTime
}

// appendCurrentTaskRecordCollectionsFromModel writes the repeated fields that
// live on the shared layout record. Dependencies are not among them: they were
// added by a later schema and are set on the record that owns them.
func appendCurrentTaskRecordCollectionsFromModel(record *format.V6TaskRecord, task *models.Task) {
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
}

// --- stored time values --------------------------------------------------

// parseProjectDocumentDate resolves a stored date-only value as the civil date
// it names. time.ParseInLocation hands time.Date a local midnight the zone may
// not have, and its answer there is 23:00 on the day before — the right instant
// under the wrong question, and a date read back a day early. The components are
// read in UTC and the start of that date is rebuilt in location, so a stored
// date round-trips to the value that wrote it.
func parseProjectDocumentDate(value string, location *time.Location) (*time.Time, error) {
	if location == nil {
		location = time.Local
	}
	civil, err := time.ParseInLocation(checkInDateLayout, value, time.UTC)
	if err != nil {
		return nil, err
	}
	if civil.Format(checkInDateLayout) != value {
		return nil, fmt.Errorf("date %q is not a canonical %s value", value, checkInDateLayout)
	}
	day, err := util.StartOfCalendarDate(civil.Year(), civil.Month(), civil.Day(), location)
	if err != nil {
		return nil, err
	}
	return &day, nil
}

// parseProjectDocumentDueDate resolves the date-only "due" field, then folds in
// an optional "due_time" (HH:MM) so a timed due date carries its clock. A due
// date with no due_time stays at the start of its date, which is the date-only
// signal.
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

// checkInDateLayout is the stored spelling of a check-in date, which is the
// deadline's layout: one metadata line holding two date-valued fields that
// disagreed about their format would be a grammar nobody could read by eye.
const checkInDateLayout = "2006-01-02"

// parseProjectDocumentCheckInDate resolves the stored check-in date as the
// civil date it names, through the same parseProjectDocumentDate the deadline
// reads, which is the instant models.ValidateCheckInSchedule measures a schedule
// against.
func parseProjectDocumentCheckInDate(value string, location *time.Location) (*time.Time, error) {
	return parseProjectDocumentDate(value, location)
}

// formatProjectDocumentCheckInDate writes a check-in date out as the calendar
// day it falls on in its own location, empty for none. The date is read off the
// value rather than converted to UTC first, because the stored value is the day
// the schedule names and not the instant it happens to begin at.
func formatProjectDocumentCheckInDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(checkInDateLayout)
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
