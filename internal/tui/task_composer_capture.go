package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

// validateComposerParent mirrors the application-layer child placement rule so a
// rejected nesting is reported on the offending line instead of failing the whole
// submission after the fact.
func validateComposerParent(project *models.Project, parentID string) error {
	parent := project.GetTaskByID(parentID)
	if parent == nil {
		return fmt.Errorf("parent not found: %s", parentID)
	}
	if parent.Level+1 > models.MaxNestingDepth {
		return fmt.Errorf("nesting under #%d would exceed maximum depth of %d levels", parent.ShortID, models.MaxNestingDepth)
	}
	return nil
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cp := *value
	return &cp
}

func mergeComposerTags(shared, line []string) []string {
	result := make([]string, 0, len(shared)+len(line))
	seen := make(map[string]bool, len(shared)+len(line))
	for _, tag := range append(append([]string(nil), shared...), line...) {
		key := strings.ToLower(strings.TrimSpace(tag))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, tag)
	}
	return result
}

func (m *TaskComposerModel) projectByID(id string) *models.Project {
	if m.preset.Project != nil && m.preset.Project.ID == id {
		return m.preset.Project
	}
	for _, project := range m.preset.AllProjects {
		if project != nil && project.ID == id {
			return project
		}
	}
	return nil
}

func (m *TaskComposerModel) expandSingle() tea.Cmd {
	if m.preset.Project == nil {
		m.errors = []grammar.LineError{{Line: 1, Message: "project is unavailable"}}
		return nil
	}
	if m.nonblankLineCount() == 0 {
		return m.expandRequest(app.CaptureRequest{
			ProjectID:   m.preset.Project.ID,
			Priority:    m.preset.Shared.Priority,
			Tags:        append([]string(nil), m.preset.Shared.Tags...),
			Due:         cloneTime(m.preset.Shared.Due),
			ParentID:    cloneStringPtr(m.preset.Shared.ParentID),
			Destination: m.preset.defaultDestination(m.preset.Shared.ParentID),
		})
	}
	reqs, ok := m.parseCompactLines()
	if !ok {
		return nil
	}
	if len(reqs) != 1 {
		m.errors = []grammar.LineError{{Line: 1, Message: "expanded mode requires exactly one nonblank task"}}
		return nil
	}
	// The expanded form has no adoption channel, so expanding would drop the
	// references silently rather than carry them.
	if len(reqs[0].ChildIDs) > 0 {
		m.errors = []grammar.LineError{{Line: 1, Message: "expanded mode cannot adopt children; submit the line with Enter instead"}}
		return nil
	}
	// The expanded form has no pin control, so expanding would drop the value
	// silently rather than carry it.
	if reqs[0].Pinned {
		m.errors = []grammar.LineError{{Line: 1, Message: "expanded mode cannot pin; submit the line with Enter instead"}}
		return nil
	}
	// The expanded form has no context control, so expanding would drop the value
	// silently rather than carry it.
	if reqs[0].Context != "" {
		m.errors = []grammar.LineError{{Line: 1, Message: "expanded mode cannot set context; submit the line with Enter instead"}}
		return nil
	}
	return m.expandRequest(reqs[0])
}

func (m *TaskComposerModel) expandRequest(req app.CaptureRequest) tea.Cmd {
	project := m.projectByID(req.ProjectID)
	if project == nil {
		m.errors = []grammar.LineError{{Line: 1, Message: "target project is unavailable"}}
		return nil
	}
	form := NewTaskFormModelCreate(project, m.runtime, nil)
	form.draft.title = req.Title
	form.draft.priority = req.Priority
	form.draft.tags = append([]string(nil), req.Tags...)
	destination := req.Destination
	if destination == "" {
		destination = m.preset.defaultDestination(req.ParentID)
	}
	form.draft.state = destination.WorkflowState()
	form.draft.parentID = cloneStringPtr(req.ParentID)
	form.draft.dueDate = cloneTime(req.Due)
	form.draft.dueInput = formatDateInput(req.Due)
	// Seed the Extras tab's drafts so an expanded &link or ;comment; is carried
	// into the form rather than silently dropped. Each section starts expanded
	// when it has content, so the values are visible without a keystroke.
	for _, raw := range req.Attachments {
		form.draft.attachments = append(form.draft.attachments, formAttachmentDraft{value: raw})
	}
	form.attachmentsExpanded = len(form.draft.attachments) > 0
	for _, raw := range req.Comments {
		form.draft.comments = append(form.draft.comments, formCommentDraft{text: raw})
	}
	form.commentsExpanded = len(form.draft.comments) > 0
	form.titleInput.SetValue(req.Title)
	form.dueTextInput.SetValue(form.draft.dueInput)
	form.presetDestination = destination
	form.hostTitle = "Task Composer · Expanded"
	form.SetTaskFormCoordinator(m.taskForm)
	form.SetSize(m.width, m.height)
	m.expanded = form
	m.mode = ComposerExpanded
	return form.Init()
}

func (m *TaskComposerModel) cycleProject() {
	if !m.preset.AllowProjectSwitch || len(m.preset.AllProjects) < 2 || m.preset.Project == nil {
		return
	}
	index := -1
	for i, project := range m.preset.AllProjects {
		if project != nil && project.ID == m.preset.Project.ID {
			index = i
			break
		}
	}
	for offset := 1; offset <= len(m.preset.AllProjects); offset++ {
		candidate := m.preset.AllProjects[(index+offset+len(m.preset.AllProjects))%len(m.preset.AllProjects)]
		if candidate == nil || candidate.Archived {
			continue
		}
		m.preset.Project = candidate
		m.Project = candidate
		m.InitCaches()
		m.UpdatePreview()
		return
	}
}
