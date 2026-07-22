package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// handleSave is the Ctrl+S entry point. It commits pending text inputs,
// validates, and dispatches to saveCreate or saveEdit.
func (m *TaskFormModel) handleSave() tea.Cmd {
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

	// Parse due date from text input (Schedule tab, field 0).
	if err := m.parseDraftDueDate(); err != nil {
		m.err = err
		m.activeTab = formTabSchedule
		m.focusIndex = 0
		m.syncInputFocus()
		return nil
	}

	// Parse start date from text input (Schedule tab, field 1).
	if err := m.parseDraftStartDate(); err != nil {
		m.err = err
		m.activeTab = formTabSchedule
		m.focusIndex = 1
		m.syncInputFocus()
		return nil
	}

	// Parse estimated days from text input (Schedule tab, field 2).
	if err := m.parseDraftEstimatedDays(); err != nil {
		m.err = err
		m.activeTab = formTabSchedule
		m.focusIndex = 2
		m.syncInputFocus()
		return nil
	}

	if m.mode == FormModeCreate {
		return m.saveCreate()
	}
	return m.saveEdit()
}

// commitAllTextInputs flushes every live textinput/textarea value into the
// draft regardless of which field is focused. Safe to call multiple times.
func (m *TaskFormModel) commitAllTextInputs() {
	m.draft.title = strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	m.draft.dueInput = m.dueTextInput.Value()
	m.draft.startDateInput = m.startDateTextInput.Value()
	m.draft.estimatedDaysInput = m.estimatedDaysTextInput.Value()
	m.draft.url = strings.TrimSpace(m.urlTextInput.Value())
	m.draft.source = strings.TrimSpace(m.sourceTextInput.Value())
}

// parseDraftDueDate parses draft.dueInput and stores the result in draft.dueDate.
// Empty / "none" / "clear" clears the due date. Returns an error for
// unparseable input. Supports optional time-of-day suffixes (e.g. "tomorrow 5pm").
func (m *TaskFormModel) parseDraftDueDate() error {
	value := strings.TrimSpace(m.draft.dueInput)
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "clear") {
		m.draft.dueDate = nil
		m.draft.hasTime = false
		return nil
	}
	weekEndDay := m.runtime.Settings().GetWeekEndDay()
	date, hasTime, err := util.ParseRelativeDateWithTime(value, weekEndDay)
	if err != nil {
		return err
	}
	m.draft.dueDate = &date
	m.draft.hasTime = hasTime
	return nil
}

// parseDraftStartDate parses draft.startDateInput and stores the result in
// draft.startDate. Empty / "none" / "clear" clears the start date.
func (m *TaskFormModel) parseDraftStartDate() error {
	value := strings.TrimSpace(m.draft.startDateInput)
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "clear") {
		m.draft.startDate = nil
		return nil
	}
	weekEndDay := m.runtime.Settings().GetWeekEndDay()
	date, _, err := util.ParseRelativeDateWithTime(value, weekEndDay)
	if err != nil {
		return err
	}
	m.draft.startDate = &date
	return nil
}

// parseDraftEstimatedDays parses draft.estimatedDaysInput and stores the
// result in draft.estimatedDays. Empty / "none" / "clear" resets to 0.
// Returns an error if the value is not a valid non-negative number.
func (m *TaskFormModel) parseDraftEstimatedDays() error {
	value := strings.TrimSpace(m.draft.estimatedDaysInput)
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "clear") {
		m.draft.estimatedDays = 0
		return nil
	}
	days, err := strconv.ParseFloat(value, 64)
	if err != nil || days < 0 {
		return fmt.Errorf("estimated days must be a non-negative number")
	}
	m.draft.estimatedDays = days
	return nil
}

// buildDraftTaskAttrs constructs the type-specific attrs pointers from draft
// state. Only the pointer for the active task type is returned non-nil.
func (m *TaskFormModel) buildDraftTaskAttrs() (
	bugAttrs *models.BugAttrs,
	ideaAttrs *models.IdeaAttrs,
	taskAttrs *models.TaskAttrs,
	storyAttrs *models.StoryAttrs,
	epicAttrs *models.EpicAttrs,
) {
	switch m.draft.taskType {
	case models.TaskTypeBug:
		a := m.draft.bugAttrs
		if !a.IsEmpty() {
			bugAttrs = &a
		}
	case models.TaskTypeIdea:
		a := m.draft.ideaAttrs
		if !a.IsEmpty() {
			ideaAttrs = &a
		}
	case models.TaskTypeTask:
		a := m.draft.taskAttrs
		if !a.IsEmpty() {
			taskAttrs = &a
		}
	case models.TaskTypeStory:
		a := m.draft.storyAttrs
		if !a.IsEmpty() {
			storyAttrs = &a
		}
	case models.TaskTypeEpic:
		a := m.draft.epicAttrs
		if !a.IsEmpty() {
			epicAttrs = &a
		}
	}
	return
}

// saveCreate builds an AddTaskRequest from the draft and routes creation through
// the task-form coordinator, which commits the new task (and any subtasks and
// template artifact) as one atomic ChangeSet. The returned Cmd emits a
// TaskCreatedMsg carrying the committed result; the shell reloads and re-resolves
// the task pointers for navigation, status, and open-after.
func (m *TaskFormModel) saveCreate() tea.Cmd {
	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildDraftTaskAttrs()

	// Build the subtask request list (blank titles skipped) and map the draft
	// open-after index onto the created (non-blank) subtask index.
	var subReqs []app.AddSubtaskRequest
	openCreatedIdx := -1
	if formTypeCanHaveSubtasks(m.draft.taskType) {
		wantOpen := m.subtaskOpenDetail || m.subtaskOpenContent
		created := 0
		for i, st := range m.draft.subtasks {
			title := strings.TrimSpace(st.title)
			if title == "" {
				continue
			}
			subReqs = append(subReqs, app.AddSubtaskRequest{Title: title, Type: st.taskType})
			if wantOpen && i == m.subtaskOpenIdx {
				openCreatedIdx = created
			}
			created++
		}
	}

	req := app.AddTaskRequest{
		ProjectID:          m.project.ID,
		Title:              m.draft.title,
		Type:               m.draft.taskType,
		Priority:           m.draft.priority,
		Tags:               append([]string{}, m.draft.tags...),
		Description:        strings.TrimSpace(m.draft.desc),
		Section:            m.draft.section,
		Recurrence:         m.draft.recurrence,
		ParentID:           m.draft.parentID,
		ChecklistItems:     append([]models.ChecklistItem(nil), m.draft.checklist...),
		DueDate:            m.draft.dueDate,
		HasTime:            m.draft.hasTime,
		StartDate:          m.draft.startDate,
		EstimatedDays:      m.draft.estimatedDays,
		URL:                m.draft.url,
		Source:             m.draft.source,
		Pinned:             m.draft.pinned,
		Dimmed:             m.draft.dimmed,
		InsertBeforeDimmed: true,
		Subtasks:           subReqs,
		BugAttrs:           bugAttrs,
		IdeaAttrs:          ideaAttrs,
		TaskAttrs:          taskAttrs,
		StoryAttrs:         storyAttrs,
		EpicAttrs:          epicAttrs,
		InitialState:       m.presetState,
		PlanningArtifacts:  m.presetPlan,
	}

	// Increment template use count when created from a template. This is a config
	// write (not a tasks.md save), left as a separate persistence step exactly as
	// the legacy path had it.
	if m.fromTemplate != nil {
		if err := commitAutomationToRuntime(m.runtime, func(automation *config.AutomationConfig) {
			automation.IncrementTemplateUseCount(m.fromTemplate.ID)
		}); err != nil {
			logging.Warn("failed to save automation after template use: %v", err)
		}
	}

	openContent, openDetail := false, false
	if openCreatedIdx >= 0 {
		openContent = m.subtaskOpenContent
		openDetail = m.subtaskOpenDetail
		m.subtaskOpenContent = false
		m.subtaskOpenDetail = false
	}

	return m.taskForm.CreateForm(req, openCreatedIdx, openContent, openDetail)
}

// readPastedClipboard reads and parses the clipboard into a util.PastedTask
// without mutating any project. It is the parse-only seam shared by the
// in-memory single-project paste (newInboxTaskFromClipboard) and the
// cross-project paste, which builds an app.CaptureRequest instead of appending a
// task directly. Returns an error when the clipboard is empty or unreadable.
func readPastedClipboard() (util.PastedTask, error) {
	text, err := clipboard.ReadAll()
	if err != nil || strings.TrimSpace(text) == "" {
		return util.PastedTask{}, fmt.Errorf("clipboard is empty")
	}
	pt := util.ParseClipboard(text)
	if pt.Title == "" {
		return util.PastedTask{}, fmt.Errorf("clipboard is empty")
	}
	return pt, nil
}

// newInboxTaskFromClipboard reads the clipboard, parses it, builds a Task in
// the project's Inbox, and appends it to project.Tasks. Callers are responsible
// for persisting the project. Returns the created task, or an error if the
// clipboard is empty or unreadable.
func newInboxTaskFromClipboard(project *models.Project) (*models.Task, error) {
	pt, err := readPastedClipboard()
	if err != nil {
		return nil, err
	}

	if !project.HasSection(models.SectionInbox) {
		project.EnsureSection(models.SectionInbox)
		project.SetSectionColor(models.SectionInbox, "red")
	}

	now := time.Now()
	task := &models.Task{
		ID:              uuid.New().String(),
		ShortID:         project.GetNextShortID(),
		Type:            models.TaskTypeTask,
		Title:           pt.Title,
		Description:     pt.Description,
		WorkflowState:   models.StateReady,
		Priority:        3,
		Tags:            []string{},
		DependsOn:       []string{},
		RelatedTo:       []string{},
		ScoreDimensions: map[string]int{},
		Children:        []*models.Task{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	task.SetSection(models.SectionInbox)

	if pt.URL != "" {
		if attachErr := task.AddAttachment(pt.URL, true, pt.URL); attachErr != nil {
			logging.Warn("clipboard paste: failed to add URL attachment: %v", attachErr)
		}
	}

	project.Tasks = append([]*models.Task{task}, project.Tasks...)
	project.BuildTaskTree()
	project.ReorderRootTasksByCompletion()

	return task, nil
}

// saveEdit builds a full-field EditRequest from the draft and routes the edit
// through the task-form coordinator, which commits the change as one atomic
// ChangeSet. The returned Cmd emits a TaskUpdatedMsg carrying the committed
// result; the shell reloads and re-resolves the live task pointer.
func (m *TaskFormModel) saveEdit() tea.Cmd {
	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildDraftTaskAttrs()

	form := &app.FormEdit{
		Title:         m.draft.title,
		Type:          m.draft.taskType,
		Priority:      m.draft.priority,
		ParentID:      m.draft.parentID,
		Section:       m.draft.section,
		Description:   strings.TrimSpace(m.draft.desc),
		Tags:          append([]string{}, m.draft.tags...),
		DueDate:       m.draft.dueDate,
		HasTime:       m.draft.hasTime,
		StartDate:     m.draft.startDate,
		EstimatedDays: m.draft.estimatedDays,
		URL:           m.draft.url,
		Source:        m.draft.source,
		Dimmed:        m.draft.dimmed,
		Pinned:        m.draft.pinned,
		Recurrence:    m.draft.recurrence,
		Checklist:     append([]models.ChecklistItem(nil), m.draft.checklist...),
		BugAttrs:      bugAttrs,
		IdeaAttrs:     ideaAttrs,
		TaskAttrs:     taskAttrs,
		StoryAttrs:    storyAttrs,
		EpicAttrs:     epicAttrs,
	}

	return m.taskForm.EditForm(taskIdentityFor(m.task, m.project), app.EditRequest{Form: form})
}

// SetTaskFormCoordinator injects the shared task-form coordinator (taskFormWireable).
func (m *TaskFormModel) SetTaskFormCoordinator(c TaskFormCoordinator) { m.taskForm = c }
