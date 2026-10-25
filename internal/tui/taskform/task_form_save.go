package taskform

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/util"
)

// handleSave is the Ctrl+S entry point. It commits pending text inputs,
// validates, and dispatches to saveCreate.
func (m *TaskFormModel) handleSave() tea.Cmd {
	if m.submissionInFlight {
		return nil
	}
	m.commitAllTextInputs()
	m.err = nil

	// Validate title (Basics tab, field 0).
	if err := models.ValidateTaskTitle(m.draft.title); err != nil {
		m.err = err
		m.activeTab = formTabBasics
		m.focusIndex = 0
		m.syncInputFocus()
		return nil
	}

	// Normalize the context (Basics tab, field 7). The application layer applies
	// the same rule; mirroring it here is what keeps a rejected value on the
	// offending row with the draft still open instead of reporting it after the
	// fact.
	if err := m.normalizeDraftContext(); err != nil {
		m.err = err
		m.activeTab = formTabBasics
		m.collapsedBasics = false
		m.focusIndex = formBasicsContextField
		m.syncInputFocus()
		return nil
	}

	// Parse due date from text input (Schedule tab, field 0).
	if err := m.parseDraftDueDate(); err != nil {
		m.err = err
		m.activeTab = formTabSchedule
		m.focusIndex = 0
		m.syncInputFocus()
		return nil
	}

	cmd := m.saveCreate()
	if m.guardSubmit != nil {
		cmd = m.guardSubmit(cmd)
	}
	if cmd != nil {
		m.submissionInFlight = true
	}
	return cmd
}

// commitAllTextInputs flushes every live textinput/textarea value into the
// draft regardless of which field is focused. Safe to call multiple times.
func (m *TaskFormModel) commitAllTextInputs() {
	m.draft.title = strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	m.draft.context = strings.TrimSpace(m.contextInput.Value())
	m.draft.dueInput = m.dueTextInput.Value()
}

// normalizeDraftContext rewrites draft.context into its canonical spelling.
// An empty field is the cleared state, so it normalizes to the unset value
// rather than an error; a non-empty one must satisfy the stored contract.
func (m *TaskFormModel) normalizeDraftContext() error {
	value := strings.TrimSpace(m.draft.context)
	if value == "" {
		m.draft.context = ""
		m.contextInput.SetValue("")
		return nil
	}
	normalized, err := models.NormalizeTaskContext(value)
	if err != nil {
		return err
	}
	m.draft.context = normalized
	m.contextInput.SetValue(normalized)
	return nil
}

// parseDraftDueDate parses draft.dueInput and stores the result in draft.dueDate.
// Empty / "none" / "clear" clears the due date. Returns an error for
// unparseable input. Supports optional time-of-day suffixes (e.g. "tomorrow 5pm").
func (m *TaskFormModel) parseDraftDueDate() error {
	value := strings.TrimSpace(m.draft.dueInput)
	// An unchanged field already carries a resolved instant. Do not round it or
	// reinterpret it in a different timezone merely because the form was saved.
	if m.draft.dueDate != nil && value == formatDateInput(m.draft.dueDate) {
		return nil
	}
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "clear") {
		m.draft.dueDate = nil
		return nil
	}
	datePart, hour, minute, hasTime, err := util.ExtractDueClockSuffix(value)
	if err != nil {
		return err
	}
	date, err := util.ParseRelativeDateWithConfig(datePart, "sun")
	if err != nil {
		return err
	}
	if hasTime {
		date = time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location())
	}
	m.draft.dueDate = &date
	return nil
}

// saveCreate builds an AddTaskRequest from the draft and routes creation through
// the task-form coordinator, which commits the new task and any subtasks as one
// atomic ChangeSet. The returned Cmd emits a
// coordinator.TaskCreatedMsg carrying the committed result; the shell reloads and re-resolves
// the task pointers for navigation, status, and open-after.
func (m *TaskFormModel) saveCreate() tea.Cmd {
	destination := app.CaptureDestination(m.draft.state)
	if m.presetDestination.IsPlanning() && m.draft.state == models.StateBacklog {
		destination = m.presetDestination
	}
	if coordinator.DueDateIsToday(m.draft.dueDate) && (m.presetDestination == "" || !sameFormDate(m.draft.dueDate, m.presetDue)) {
		destination = app.CaptureDestinationToday
	}

	// Build the subtask request list (blank titles skipped) and map the draft
	// open-after index onto the created (non-blank) subtask index.
	var subReqs []app.AddSubtaskRequest
	openCreatedIdx := -1
	wantOpen := m.subtaskOpenContent
	created := 0
	for i, st := range m.draft.subtasks {
		title := strings.TrimSpace(st.title)
		if title == "" {
			continue
		}
		subReqs = append(subReqs, app.AddSubtaskRequest{Title: title})
		if wantOpen && i == m.subtaskOpenIdx {
			openCreatedIdx = created
		}
		created++
	}

	// Build the attachment request list (blank values skipped), so the drafts
	// ride the same ChangeSet as the task write and one u undoes both. The raw
	// value is passed through verbatim: classification, resolution, and the
	// staged file copy all belong to the application layer, exactly as they do
	// for a captured &link.
	var attachmentReqs []string
	for _, att := range m.draft.attachments {
		value := strings.TrimSpace(att.value)
		if value == "" {
			continue
		}
		attachmentReqs = append(attachmentReqs, value)
	}

	// Build the comment request list the same way (blank texts skipped), so the
	// task, its attachments, and its comments all commit in one ChangeSet under
	// one undo entry. normalizeCommentText runs in the application layer, on the
	// same path a captured ;comment; takes.
	var commentReqs []string
	for _, comment := range m.draft.comments {
		text := strings.TrimSpace(comment.text)
		if text == "" {
			continue
		}
		commentReqs = append(commentReqs, text)
	}

	req := app.AddTaskRequest{
		Prepend:        m.draft.prepend,
		DeferredUntil:  cloneTime(m.draft.deferredUntil),
		ChildIDs:       append([]string(nil), m.draft.childIDs...),
		Dependencies:   append([]app.TaskLocator(nil), m.draft.dependencies...),
		ProjectID:      m.project.ID,
		Title:          m.draft.title,
		Priority:       m.draft.priority,
		Tags:           append([]string{}, m.draft.tags...),
		Context:        m.draft.context,
		Description:    strings.TrimSpace(m.draft.desc),
		Destination:    destination,
		ParentID:       m.draft.parentID,
		ChecklistItems: append([]models.ChecklistItem(nil), m.draft.checklist...),
		DueDate:        m.draft.dueDate,
		CheckIn:        m.draft.checkIn.Clone(),
		Pinned:         m.draft.pinned,
		Review:         m.draft.review,
		Subtasks:       subReqs,
		Attachments:    attachmentReqs,
		Comments:       commentReqs,
	}

	openContent := false
	if openCreatedIdx >= 0 {
		openContent = m.subtaskOpenContent
		m.subtaskOpenContent = false
	}

	return m.taskForm.CreateForm(req, openCreatedIdx, openContent)
}

// SetTaskFormCoordinator injects the shared task-form coordinator (taskFormWireable).
func (m *TaskFormModel) SetTaskFormCoordinator(c coordinator.TaskFormCoordinator) { m.taskForm = c }

func sameFormDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
