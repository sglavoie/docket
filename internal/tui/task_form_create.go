package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

// NewTaskFormModelCreate creates a TaskFormModel for creating a new task.
func NewTaskFormModelCreate(
	project *models.Project,
	cfg *config.Config,
	mgr *config.Manager,
	suggestedParent *models.Task,
) *TaskFormModel {
	draft := taskFormDraft{
		taskType: models.TaskTypeTask,
		priority: 3,
		section:  models.SectionNext,
	}
	if suggestedParent != nil {
		// Walk up to find the nearest ancestor that can accept this child type
		candidate := suggestedParent
		for candidate != nil {
			if candidate.CanAddChild(draft.taskType) {
				id := candidate.ID
				draft.parentID = &id
				break
			}
			candidate = candidate.Parent
		}
	}

	var collapsed bool
	if cfg != nil {
		collapsed = cfg.GetTaskFormCollapsed()
	}
	m := &TaskFormModel{
		mode:              FormModeCreate,
		project:           project,
		draft:             draft,
		config:            cfg,
		configManager:     mgr,
		activeTab:         formTabBasics,
		focusIndex:        0,
		collapsedBasics:   collapsed,
		collapsedSchedule: collapsed,
		width:             80,
		height:            24,
	}
	m.initTitleInput("")
	m.initDueInput("")
	m.initStartDateInput("")
	m.initEstimatedDaysInput("")
	m.initURLInput("")
	m.initSourceInput("")
	m.initChecklistItemInput("")
	m.initSubtaskItemInput("")
	return m
}

// NewTaskFormModelCreateFromTemplate creates a TaskFormModel pre-filled from a template.
func NewTaskFormModelCreateFromTemplate(
	project *models.Project,
	cfg *config.Config,
	mgr *config.Manager,
	suggestedParent *models.Task,
	tmpl *template.TaskTemplate,
) *TaskFormModel {
	m := NewTaskFormModelCreate(project, cfg, mgr, suggestedParent)
	m.fromTemplate = tmpl

	if tmpl == nil {
		return m
	}

	m.draft.taskType = tmpl.Type
	if tmpl.Priority > 0 {
		m.draft.priority = tmpl.Priority
	}
	if len(tmpl.Tags) > 0 {
		m.draft.tags = append([]string{}, tmpl.Tags...)
	}
	if tmpl.Section != "" {
		project.EnsureSection(tmpl.Section)
		m.draft.section = tmpl.Section
	}
	if tmpl.Description != "" {
		m.draft.desc = tmpl.Description
	}

	if len(tmpl.Checklist) > 0 {
		for _, item := range tmpl.Checklist {
			m.draft.checklist = append(m.draft.checklist, models.ChecklistItem{
				Text: item,
				Done: false,
			})
		}
	} else if len(tmpl.Children) > 0 {
		for _, child := range tmpl.Children {
			m.draft.checklist = append(m.draft.checklist, models.ChecklistItem{
				Text: child.Title,
				Done: false,
			})
		}
	}

	return m
}

// NewTaskFormModelEdit creates a TaskFormModel for editing an existing task.
func NewTaskFormModelEdit(
	project *models.Project,
	task *models.Task,
	cfg *config.Config,
	mgr *config.Manager,
) *TaskFormModel {
	// Resolve the effective section: custom sections are stored in
	// CustomSection (with Section=""), so we must pick the right one
	// to avoid losing the custom section on save via SetSection("").
	effectiveSection := task.Section
	if effectiveSection == "" && task.CustomSection != "" {
		effectiveSection = task.CustomSection
	}

	draft := taskFormDraft{
		taskType:           task.Type,
		priority:           task.Priority,
		parentID:           task.ParentID,
		section:            effectiveSection,
		pinned:             task.Pinned,
		active:             task.Active,
		dimmed:             task.Dimmed,
		tags:               append([]string{}, task.Tags...),
		dueDate:            cloneTime(task.DueDate),
		dueInput:           formatDateInput(task.DueDate, task.HasTime),
		hasTime:            task.HasTime,
		recurrence:         cloneRecurrence(task.Recurrence),
		startDate:          cloneTime(task.StartDate),
		startDateInput:     formatDateInput(task.StartDate, false),
		estimatedDays:      task.EstimatedDays,
		estimatedDaysInput: formatEstimatedDaysInput(task.EstimatedDays),
		desc:               task.Description,
		url:                task.URL,
		source:             task.Source,
		title:              task.Title,
		checklist:          append([]models.ChecklistItem(nil), task.Checklist...),
	}

	// Load type-specific attrs from the task.
	if task.BugAttrs != nil {
		draft.bugAttrs = *task.BugAttrs
	}
	if task.IdeaAttrs != nil {
		draft.ideaAttrs = *task.IdeaAttrs
	}
	if task.TaskAttrs != nil {
		draft.taskAttrs = *task.TaskAttrs
	}
	if task.StoryAttrs != nil {
		draft.storyAttrs = *task.StoryAttrs
	}
	if task.EpicAttrs != nil {
		draft.epicAttrs = *task.EpicAttrs
	}

	var collapsed bool
	if cfg != nil {
		collapsed = cfg.GetTaskFormCollapsed()
	}
	m := &TaskFormModel{
		mode:              FormModeEdit,
		project:           project,
		task:              task,
		draft:             draft,
		config:            cfg,
		configManager:     mgr,
		activeTab:         formTabBasics,
		focusIndex:        0,
		collapsedBasics:   collapsed,
		collapsedSchedule: collapsed,
		width:             80,
		height:            24,
	}
	m.initTitleInput(draft.title)
	m.initDueInput(draft.dueInput)
	m.initStartDateInput(draft.startDateInput)
	m.initEstimatedDaysInput(draft.estimatedDaysInput)
	m.initURLInput(draft.url)
	m.initSourceInput(draft.source)
	m.initChecklistItemInput("")
	m.initSubtaskItemInput("")
	return m
}

// cloneTime returns a deep copy of a *time.Time, or nil if the input is nil.
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

// cloneRecurrence returns a deep copy of a *models.RecurrencePattern, or nil.
// The Weekdays slice is also copied so mutations on the clone do not alias
// the original task's slice (and vice versa).
func cloneRecurrence(value *models.RecurrencePattern) *models.RecurrencePattern {
	if value == nil {
		return nil
	}
	cp := *value
	if len(value.Weekdays) > 0 {
		cp.Weekdays = append([]int(nil), value.Weekdays...)
	}
	return &cp
}
