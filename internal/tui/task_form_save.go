package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/google/uuid"

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
	weekEndDay := "sun"
	if m.config != nil {
		weekEndDay = m.config.GetWeekEndDay()
	}
	date, hasTime, err := util.ParseRelativeDateWithTime(value, weekEndDay)
	if err != nil {
		return err
	}
	m.draft.dueDate = &date
	m.draft.hasTime = hasTime
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

// saveCreate creates a new task from the draft, appends subtasks, saves the
// project, and returns a Cmd that emits TaskCreatedMsg.
func (m *TaskFormModel) saveCreate() tea.Cmd {
	now := time.Now()
	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildDraftTaskAttrs()

	task := &models.Task{
		ID:              uuid.New().String(),
		ShortID:         m.project.GetNextShortID(),
		Type:            m.draft.taskType,
		Title:           m.draft.title,
		Description:     strings.TrimSpace(m.draft.desc),
		Status:          models.StatusTodo,
		Priority:        m.draft.priority,
		ParentID:        m.draft.parentID,
		DueDate:         m.draft.dueDate,
		HasTime:         m.draft.hasTime,
		Pinned:          m.draft.pinned,
		Active:          m.draft.active,
		Dimmed:          m.draft.dimmed,
		Recurrence:      m.draft.recurrence,
		URL:             m.draft.url,
		Source:          m.draft.source,
		BugAttrs:        bugAttrs,
		IdeaAttrs:       ideaAttrs,
		TaskAttrs:       taskAttrs,
		StoryAttrs:      storyAttrs,
		EpicAttrs:       epicAttrs,
		DependsOn:       []string{},
		RelatedTo:       []string{},
		ScoreDimensions: map[string]int{},
		Children:        []*models.Task{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	task.SetSection(m.draft.section)

	if m.draft.pinned {
		task.PinnedAt = &now
	}

	task.Tags = append([]string{}, m.draft.tags...)

	// Build checklist from draft items (skip blank entries).
	for _, item := range m.draft.checklist {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		task.Checklist = append(task.Checklist, models.ChecklistItem{
			Text: text,
			Done: item.Done,
		})
	}

	m.project.Tasks = append(m.project.Tasks, task)

	// Create subtask children for Epic/Story (create mode only).
	if formTypeCanHaveSubtasks(m.draft.taskType) {
		for _, st := range m.draft.subtasks {
			stTitle := strings.TrimSpace(st.title)
			if stTitle == "" {
				continue
			}
			subtask := &models.Task{
				ID:              uuid.New().String(),
				ShortID:         m.project.GetNextShortID(),
				Type:            st.taskType,
				Title:           stTitle,
				Status:          models.StatusTodo,
				Priority:        m.draft.priority,
				ParentID:        &task.ID,
				DependsOn:       []string{},
				RelatedTo:       []string{},
				ScoreDimensions: map[string]int{},
				Children:        []*models.Task{},
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			subtask.SetSection(m.draft.section)
			subtask.Tags = append([]string{}, m.draft.tags...)
			m.project.Tasks = append(m.project.Tasks, subtask)
		}
	}

	// Increment template use count when created from a template.
	if m.fromTemplate != nil && m.config != nil && m.configManager != nil {
		m.config.IncrementTemplateUseCount(m.fromTemplate.ID)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config after template use: %v", err)
		}
	}

	m.project.BuildTaskTree()
	m.project.ReorderRootTasksByCompletion()

	if err := saveProjectSync(m.project); err != nil {
		m.err = fmt.Errorf("failed to save project: %w", err)
		return nil
	}

	return func() tea.Msg { return TaskCreatedMsg{Task: task} }
}

// newInboxTaskFromClipboard reads the clipboard, parses it, builds a Task in
// the project's Inbox, and appends it to project.Tasks. Callers are responsible
// for persisting the project. Returns the created task, or an error if the
// clipboard is empty or unreadable.
func newInboxTaskFromClipboard(project *models.Project) (*models.Task, error) {
	text, err := clipboard.ReadAll()
	if err != nil || strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("clipboard is empty")
	}

	pt := util.ParseClipboard(text)
	if pt.Title == "" {
		return nil, fmt.Errorf("clipboard is empty")
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
		Status:          models.StatusTodo,
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

// saveEdit applies draft changes to the existing task via MutateTask, saves
// the project, and returns a Cmd that emits TaskUpdatedMsg.
func (m *TaskFormModel) saveEdit() tea.Cmd {
	bugAttrs, ideaAttrs, taskAttrs, storyAttrs, epicAttrs := m.buildDraftTaskAttrs()

	// Capture draft and task reference before the closure (avoids capturing m).
	draft := m.draft
	task := m.task

	m.project.MutateTask(task.ID, func(t *models.Task) {
		t.Title = draft.title
		t.Type = draft.taskType
		t.Priority = draft.priority
		t.ParentID = draft.parentID
		t.SetSection(draft.section)
		t.DueDate = draft.dueDate
		t.HasTime = draft.hasTime
		t.Tags = append([]string{}, draft.tags...)
		t.Active = draft.active
		t.Dimmed = draft.dimmed
		t.Recurrence = draft.recurrence
		t.URL = draft.url
		t.Source = draft.source
		t.Description = strings.TrimSpace(draft.desc)
		// Filter blank checklist items (consistent with saveCreate and the parser,
		// which both skip empty-text items).
		t.Checklist = nil
		for _, item := range draft.checklist {
			text := strings.TrimSpace(item.Text)
			if text != "" {
				t.Checklist = append(t.Checklist, models.ChecklistItem{
					Text: text,
					Done: item.Done,
				})
			}
		}

		t.BugAttrs = bugAttrs
		t.IdeaAttrs = ideaAttrs
		t.TaskAttrs = taskAttrs
		t.StoryAttrs = storyAttrs
		t.EpicAttrs = epicAttrs

		// Update PinnedAt only when the pinned flag actually changes.
		if draft.pinned != t.Pinned {
			t.Pinned = draft.pinned
			if draft.pinned {
				now := time.Now()
				t.PinnedAt = &now
			} else {
				t.PinnedAt = nil
			}
		}
	})

	m.project.BuildTaskTree()

	if err := saveProjectSync(m.project); err != nil {
		m.err = fmt.Errorf("failed to save project: %w", err)
		return nil
	}

	return func() tea.Msg { return TaskUpdatedMsg{Task: task} }
}
