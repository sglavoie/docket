package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// NewTaskEditorModel creates a new task editor.
func NewTaskEditorModel(project *models.Project, task *models.Task, cfg *config.Config, manager *config.Manager) *TaskEditorModel {
	factory := DefaultFormFieldFactory()

	titleInput := factory.NewTitleInputWithValue(task.Title)
	titleInput.Focus()
	titleInput.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })

	types := []models.TaskType{
		models.TaskTypeTask,
		models.TaskTypeStory,
		models.TaskTypeEpic,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	m := &TaskEditorModel{
		project:            project,
		task:               task,
		config:             cfg,
		configManager:      manager,
		originalTitle:      task.Title,
		originalTags:       strings.Join(task.Tags, ", "),
		originalType:       task.Type,
		originalParentID:   task.ParentID,
		originalPriority:   task.Priority,
		originalSection:    task.Section,
		originalDueDate:    cloneTime(task.DueDate),
		originalPinned:     task.Pinned,
		originalActive:     task.Active,
		originalDimmed:     task.Dimmed,
		originalRecurrence: cloneRecurrence(task.Recurrence),
		titleInput:         titleInput,
		tags:               append([]string{}, task.Tags...),
		taskType:           task.Type,
		typeCursor:         findTaskTypeCursor(types, task.Type),
		types:              types,
		parentID:           task.ParentID,
		priority:           task.Priority,
		section:            task.Section,
		sectionCursor:      findSectionCursor(project.GetSections(), task.Section),
		sections:           project.GetSections(),
		dueDate:            cloneTime(task.DueDate),
		dueDateInput:       formatDateInput(task.DueDate),
		pinned:             task.Pinned,
		active:             task.Active,
		dimmed:             task.Dimmed,
		flagCursor:         flagCursorFromFlags(task.Pinned, task.Active, task.Dimmed),
		recurrence:         cloneRecurrence(task.Recurrence),
		recurrenceCursor:   findRecurrenceCursor(task.Recurrence),
		recurrenceOpts:     []string{"None", "Daily", "Weekly", "Biweekly", "Monthly", "Yearly", "Custom..."},
		urlInput:           task.URL,
		sourceInput:        task.Source,
		originalURL:        task.URL,
		originalSource:     task.Source,
		focusIndex:         0,
		width:              80,
		height:             24,
	}

	m.loadExistingAttrs(task)
	m.updateValidParents()

	return m
}

// SetSize sets the view dimensions.
func (m *TaskEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextarea(&m.titleInput, width)
}

func (m *TaskEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneRecurrence(value *models.RecurrencePattern) *models.RecurrencePattern {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func findTaskTypeCursor(types []models.TaskType, current models.TaskType) int {
	for i, taskType := range types {
		if taskType == current {
			return i
		}
	}
	return 0
}

func findSectionCursor(sections []string, current string) int {
	for i, section := range sections {
		if section == current {
			return i
		}
	}
	return 0
}

func formatDateInput(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func findRecurrenceCursor(pattern *models.RecurrencePattern) int {
	if pattern == nil {
		return 0
	}

	switch {
	case pattern.Unit == models.RecurrenceDaily && pattern.Interval == 1 && len(pattern.Weekdays) == 0:
		return 1
	case pattern.Unit == models.RecurrenceWeekly && pattern.Interval == 1 && len(pattern.Weekdays) == 0:
		return 2
	case pattern.Unit == models.RecurrenceWeekly && pattern.Interval == 2 && len(pattern.Weekdays) == 0:
		return 3
	case pattern.Unit == models.RecurrenceMonthly && pattern.Interval == 1:
		return 4
	case pattern.Unit == models.RecurrenceYearly && pattern.Interval == 1:
		return 5
	default:
		return 6
	}
}
