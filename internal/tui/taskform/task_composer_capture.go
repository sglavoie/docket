package taskform

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cp := *value
	return &cp
}

func (m *TaskComposerModel) projectByID(id string) *models.Project {
	if m.Preset.Project != nil && m.Preset.Project.ID == id {
		return m.Preset.Project
	}
	for _, project := range m.Preset.AllProjects {
		if project != nil && project.ID == id {
			return project
		}
	}
	return nil
}

func (m *TaskComposerModel) ExpandSingle() tea.Cmd {
	if m.Preset.Project == nil {
		m.Errors = []grammar.LineError{{Line: 1, Message: "project is unavailable"}}
		return nil
	}
	if m.nonblankLineCount() == 0 {
		return m.ExpandRequest(app.CaptureRequest{
			ProjectID:   m.Preset.Project.ID,
			Priority:    m.Preset.Shared.Priority,
			Tags:        append([]string(nil), m.Preset.Shared.Tags...),
			Due:         cloneTime(m.Preset.Shared.Due),
			ParentID:    cloneStringPtr(m.Preset.Shared.ParentID),
			Destination: m.Preset.defaultDestination(m.Preset.Shared.ParentID),
		})
	}
	reqs, ok := m.ParseCompactLines()
	if !ok {
		return nil
	}
	if len(reqs) != 1 {
		m.Errors = []grammar.LineError{{Line: 1, Message: "expanded mode requires exactly one nonblank task"}}
		return nil
	}
	return m.ExpandRequest(reqs[0])
}

func (m *TaskComposerModel) ExpandRequest(req app.CaptureRequest) tea.Cmd {
	project := m.projectByID(req.ProjectID)
	if project == nil {
		m.Errors = []grammar.LineError{{Line: 1, Message: "target project is unavailable"}}
		return nil
	}
	form := NewTaskFormModelCreate(project, m.runtime, nil)
	form.allProjects = append([]*models.Project(nil), m.Preset.AllProjects...)
	form.draft.prepend = req.Prepend
	form.draft.deferredUntil = cloneTime(req.Defer)
	form.draft.childIDs = append([]string(nil), req.ChildIDs...)
	form.draft.dependencies = append([]app.TaskLocator(nil), req.Dependencies...)
	form.presetDue = cloneTime(req.Due)
	form.draft.title = req.Title
	form.draft.priority = req.Priority
	form.draft.tags = append([]string(nil), req.Tags...)
	form.draft.context = req.Context
	form.draft.pinned = req.Pinned
	form.draft.review = req.Review
	form.draft.desc = req.Description
	destination := req.Destination
	if destination == "" {
		destination = m.Preset.defaultDestination(req.ParentID)
	}
	form.draft.state = destination.WorkflowState()
	form.draft.parentID = cloneStringPtr(req.ParentID)
	form.draft.dueDate = cloneTime(req.Due)
	form.draft.dueInput = formatDateInput(req.Due)
	form.draft.checkIn = req.CheckIn.Clone()
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
	form.hasChanges = req.Title != ""
	form.titleInput.SetValue(req.Title)
	form.contextInput.SetValue(req.Context)
	form.dueTextInput.SetValue(form.draft.dueInput)
	form.presetDestination = destination
	form.hostTitle = "Task Composer · Expanded"
	form.SetTaskFormCoordinator(m.taskForm)
	form.SetSize(m.width, m.height)
	form.SetShellFrameMetrics(m.shellFrame)
	m.Expanded = form
	m.Mode = ComposerExpanded
	m.Errors = nil
	return form.Init()
}

// seedExpandedForm builds the expanded form from a typed seed and leaves the
// composer in expanded mode. It is ExpandRequest's sibling and deliberately not
// a call into it: an expansion starts from a parsed capture line, while a seed
// starts from values that must never meet the parser, so the two share a shape
// rather than a path.
//
// Fields the seed does not carry are left at their created-draft defaults, which
// is what makes the absences durable: no parent, no due date, no deferral, no
// pin, no review, no subtask, no attachment, no comment, no dependency.
func (m *TaskComposerModel) seedExpandedForm(seed TaskComposerSeed) {
	project := m.Preset.Project
	if project == nil {
		// initCompactInput already recorded the missing-project refusal, and the
		// compact surface can still report it; an expanded form cannot.
		return
	}
	form := NewTaskFormModelCreate(project, m.runtime, nil)
	form.draft.title = seed.Title
	form.draft.desc = seed.Description
	form.draft.priority = seed.Priority
	form.draft.tags = append([]string(nil), seed.Tags...)
	form.draft.context = seed.Context
	destination := seed.Destination
	if destination == "" {
		destination = app.CaptureDestinationBacklog
	}
	form.draft.state = destination.WorkflowState()
	for _, text := range seed.ChecklistItems {
		form.draft.checklist = append(form.draft.checklist, models.ChecklistItem{Text: text, Done: false})
	}
	form.checklistExpanded = len(form.draft.checklist) > 0
	form.hasChanges = seed.Title != ""
	form.titleInput.SetValue(seed.Title)
	form.contextInput.SetValue(seed.Context)
	form.dueTextInput.SetValue("")
	form.presetDestination = destination
	form.hostTitle = seed.HostTitle
	form.SetTaskFormCoordinator(m.taskForm)
	form.SetSize(m.width, m.height)
	form.SetShellFrameMetrics(m.shellFrame)
	m.Expanded = form
	m.Mode = ComposerExpanded
}

func (m *TaskComposerModel) cycleProject() {
	if !m.Preset.AllowProjectSwitch || len(m.Preset.AllProjects) < 2 || m.Preset.Project == nil {
		return
	}
	index := -1
	for i, project := range m.Preset.AllProjects {
		if project != nil && project.ID == m.Preset.Project.ID {
			index = i
			break
		}
	}
	for offset := 1; offset <= len(m.Preset.AllProjects); offset++ {
		candidate := m.Preset.AllProjects[(index+offset+len(m.Preset.AllProjects))%len(m.Preset.AllProjects)]
		if candidate == nil || candidate.Archived {
			continue
		}
		m.Preset.Project = candidate
		m.Project = candidate
		m.InitCaches()
		m.UpdatePreview()
		return
	}
}
