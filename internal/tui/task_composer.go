package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskComposerPresetKind is the typed intent carried into the one task composer
// overlay. Presentation names from the retired capture/form family never decide
// behavior inside the composer.
type TaskComposerPresetKind uint8

const (
	ComposerGlobal TaskComposerPresetKind = iota
	ComposerProject
	ComposerPlanner
	ComposerChild
	ComposerEdit
)

type TaskComposerMode uint8

const (
	ComposerCompact TaskComposerMode = iota
	ComposerExpanded
)

// TaskComposerSharedAttributes are defaults shared by every nonblank compact
// line. Explicit capture syntax on a line wins over these defaults.
type TaskComposerSharedAttributes struct {
	Priority   int
	Tags       []string
	Due        *time.Time
	ParentID   *string
	Recurrence *models.RecurrencePattern
	State      models.WorkflowState
}

// TaskComposerPreset is the complete typed launch contract for compact create,
// expanded create/edit, Planner membership, children, and links.
type TaskComposerPreset struct {
	Kind               TaskComposerPresetKind
	Project            *models.Project
	AllProjects        []*models.Project
	Task               *models.Task
	Parent             *models.Task
	Shared             TaskComposerSharedAttributes
	InitialText        string
	AllowProjectSwitch bool
	PreferMultiline    bool
	PlannerTab         int
	PlanningArtifacts  app.PlanningArtifactBuilder
}

func (p TaskComposerPreset) label() string {
	switch p.Kind {
	case ComposerGlobal:
		return "Global capture"
	case ComposerProject:
		return "Project task"
	case ComposerPlanner:
		switch p.PlannerTab {
		case boardTabToday:
			return "Today"
		case boardTabNext:
			return "Next"
		default:
			return "Backlog"
		}
	case ComposerChild:
		return "Child task"
	case ComposerEdit:
		return "Edit task"
	default:
		return "Task"
	}
}

func creationStateFromPreset(preset models.WorkflowState) models.WorkflowState {
	if preset != "" {
		return preset
	}
	return models.StateInbox
}

func creationStateFromParsedInput(
	state models.WorkflowState,
	stateSet bool,
	preset models.WorkflowState,
	parentID *string,
) models.WorkflowState {
	if stateSet {
		return state
	}
	if parentID != nil {
		return models.StateReady
	}
	return creationStateFromPreset(preset)
}

// TaskComposerModel is the sole live task creation/editing overlay. Compact
// mode owns one-or-many capture lines; expanded mode embeds the full form for
// exactly one task.
type lineError struct {
	line    int
	message string
}

type TaskComposerModel struct {
	quickCaptureInput
	preset   TaskComposerPreset
	runtime  *config.RuntimeState
	mode     TaskComposerMode
	expanded *TaskFormModel
	capture  CaptureCoordinator
	taskForm TaskFormCoordinator
	errors   []lineError
	width    int
	height   int
	discardConfirmState
}

func NewTaskComposerModel(preset TaskComposerPreset, runtime *config.RuntimeState) *TaskComposerModel {
	m := &TaskComposerModel{
		preset:  preset,
		runtime: runtime,
		mode:    ComposerCompact,
		width:   80,
		height:  22,
	}

	switch preset.Kind {
	case ComposerEdit:
		m.mode = ComposerExpanded
		if preset.Project != nil && preset.Task != nil {
			m.expanded = NewTaskFormModelEdit(preset.Project, preset.Task, runtime)
			m.expanded.hostTitle = "Task Composer · Edit"
		} else {
			m.errors = []lineError{{line: 1, message: "edit preset requires a task and project"}}
		}
	default:
		m.initCompactInput()
	}
	return m
}

func (m *TaskComposerModel) initCompactInput() {
	project := m.preset.Project
	placeholder := "Type a task title…"
	if m.preset.PreferMultiline {
		placeholder = "One task per line…"
	}
	m.quickCaptureInput = newQuickCaptureInput(project, m.preset.AllProjects, placeholder, !m.preset.AllowProjectSwitch)
	m.input.CharLimit = 4000
	m.input.SetHeight(5)
	if m.preset.InitialText != "" {
		m.input.SetValue(m.preset.InitialText)
		m.input.CursorEnd()
		m.updatePreview()
	}
	if project == nil {
		m.errors = []lineError{{line: 1, message: "composer preset requires a project"}}
	}
}

func (m *TaskComposerModel) SetCaptureCoordinator(c CaptureCoordinator) { m.capture = c }

func (m *TaskComposerModel) SetTaskFormCoordinator(c TaskFormCoordinator) {
	m.taskForm = c
	if m.expanded != nil {
		m.expanded.SetTaskFormCoordinator(c)
	}
}

func (m *TaskComposerModel) SetSize(width, height int) {
	m.width, m.height = width, height
	if m.expanded != nil {
		m.expanded.SetSize(width, height)
	}
	if m.mode == ComposerCompact {
		m.quickCaptureInput.width = max(width-2, 1)
		m.input.SetWidth(max(min(width-4, 90), 20))
		inputHeight := 5
		if height < 24 {
			inputHeight = 3
		}
		m.input.SetHeight(inputHeight)
		m.setSyntaxViewportWidth(max(width-2, 1))
	}
}

func (m *TaskComposerModel) Init() tea.Cmd {
	if m.expanded != nil {
		return m.expanded.Init()
	}
	return textarea.Blink
}

func (m *TaskComposerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == ComposerExpanded {
		if m.expanded == nil {
			return m, nil
		}
		updated, cmd := m.expanded.Update(msg)
		if form, ok := updated.(*TaskFormModel); ok {
			m.expanded = form
		}
		return m, cmd
	}

	if m.confirmDiscard {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
			}
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		name := key.String()
		if m.handleAutocompleteKey(name) {
			return m, nil
		}
		if m.helpExpanded && !m.isAutocompleteActive() {
			switch name {
			case "ctrl+k":
				m.syntaxScrollLine(-1)
				return m, nil
			case "ctrl+j":
				m.syntaxScrollLine(1)
				return m, nil
			case "pgup":
				m.syntaxScrollPage(-1)
				return m, nil
			case "pgdown":
				m.syntaxScrollPage(1)
				return m, nil
			}
		}
		if name == "?" && m.ac.Mode == acModeNone {
			m.toggleHelp()
			return m, nil
		}
		switch name {
		case "esc":
			if m.helpExpanded {
				m.closeHelp()
				return m, nil
			}
			if strings.TrimSpace(m.input.Value()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "shift+enter":
			m.input.InsertString("\n")
			m.updatePreview()
			return m, nil
		case "enter", "ctrl+s":
			if m.ac.Mode == acModeNone {
				return m, m.createCompact(false, false)
			}
		case "ctrl+d":
			if m.ac.Mode == acModeNone {
				return m, m.createCompact(true, false)
			}
		case "ctrl+o":
			if m.ac.Mode == acModeNone {
				return m, m.createCompact(false, true)
			}
		case "ctrl+f":
			return m, m.expandSingle()
		case "ctrl+p":
			m.cycleProject()
			return m, nil
		}
	}

	return m, m.updateInput(msg)
}

func (m *TaskComposerModel) createCompact(openContent, openDetail bool) tea.Cmd {
	reqs, ok := m.parseCompactLines()
	if !ok {
		return nil
	}
	if len(reqs) == 1 {
		return m.capture.CaptureCreate(reqs[0], openContent, openDetail)
	}
	return m.capture.CaptureBulk(reqs, true)
}

func (m *TaskComposerModel) parseCompactLines() ([]app.CaptureRequest, bool) {
	m.errors = nil
	if m.preset.Project == nil {
		m.errors = []lineError{{line: 1, message: "project is unavailable"}}
		return nil, false
	}

	lines := strings.Split(m.input.Value(), "\n")
	reqs := make([]app.CaptureRequest, 0, len(lines))
	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parsed, err := parseQuickCapture(line, m.preset.Project, m.preset.AllProjects, !m.preset.AllowProjectSwitch)
		if err != nil {
			m.errors = append(m.errors, lineError{line: idx + 1, message: err.Error()})
			continue
		}
		if err := models.ValidateTaskTitle(parsed.title); err != nil {
			m.errors = append(m.errors, lineError{line: idx + 1, message: err.Error()})
			continue
		}

		target := m.preset.Project
		if parsed.targetProjectID != nil {
			target = m.projectByID(*parsed.targetProjectID)
		}
		if target == nil {
			m.errors = append(m.errors, lineError{line: idx + 1, message: "target project is unavailable"})
			continue
		}

		priority := parsed.priority
		if m.preset.Shared.Priority != 0 && !parsed.prioritySet {
			priority = m.preset.Shared.Priority
		}
		parentID := parsed.parentID
		if parentID == nil {
			parentID = cloneStringPtr(m.preset.Shared.ParentID)
		}
		due := parsed.dueDate
		if due == nil {
			due = cloneTime(m.preset.Shared.Due)
		}
		recurrence := parsed.recurrence
		if recurrence == nil {
			recurrence = cloneRecurrence(m.preset.Shared.Recurrence)
		}
		state := creationStateFromParsedInput(parsed.state, parsed.stateSet, m.preset.Shared.State, parentID)
		req := app.CaptureRequest{
			SourceLine:        idx + 1,
			ProjectID:         target.ID,
			Title:             parsed.title,
			Priority:          priority,
			Tags:              mergeComposerTags(m.preset.Shared.Tags, parsed.tags),
			Due:               due,
			ParentID:          parentID,
			Recurrence:        recurrence,
			State:             state,
			Prepend:           true,
			PlanningArtifacts: m.preset.PlanningArtifacts,
		}
		if parentID != nil {
			if err := validateComposerParent(target, *parentID); err != nil {
				m.errors = append(m.errors, lineError{line: idx + 1, message: err.Error()})
				continue
			}
		}
		reqs = append(reqs, req)
	}

	if len(reqs) == 0 && len(m.errors) == 0 {
		m.errors = []lineError{{line: 1, message: "no tasks to create"}}
	}
	if len(m.errors) > 0 {
		return nil, false
	}
	return reqs, true
}

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
		m.errors = []lineError{{line: 1, message: "project is unavailable"}}
		return nil
	}
	if m.nonblankLineCount() == 0 {
		priority := m.preset.Shared.Priority
		if priority == 0 {
			priority = models.PriorityDefault
		}
		return m.expandRequest(app.CaptureRequest{
			ProjectID:         m.preset.Project.ID,
			Priority:          priority,
			Tags:              append([]string(nil), m.preset.Shared.Tags...),
			Due:               cloneTime(m.preset.Shared.Due),
			ParentID:          cloneStringPtr(m.preset.Shared.ParentID),
			Recurrence:        cloneRecurrence(m.preset.Shared.Recurrence),
			State:             creationStateFromPreset(m.preset.Shared.State),
			PlanningArtifacts: m.preset.PlanningArtifacts,
		})
	}
	reqs, ok := m.parseCompactLines()
	if !ok {
		return nil
	}
	if len(reqs) != 1 {
		m.errors = []lineError{{line: 1, message: "expanded mode requires exactly one nonblank task"}}
		return nil
	}
	return m.expandRequest(reqs[0])
}

func (m *TaskComposerModel) expandRequest(req app.CaptureRequest) tea.Cmd {
	project := m.projectByID(req.ProjectID)
	if project == nil {
		m.errors = []lineError{{line: 1, message: "target project is unavailable"}}
		return nil
	}
	form := NewTaskFormModelCreate(project, m.runtime, nil)
	form.draft.title = req.Title
	form.draft.priority = req.Priority
	form.draft.tags = append([]string(nil), req.Tags...)
	state := req.State
	if state == "" {
		state = models.StateInbox
	}
	form.draft.state = state
	form.draft.parentID = cloneStringPtr(req.ParentID)
	form.draft.dueDate = cloneTime(req.Due)
	form.draft.dueInput = formatDateInput(req.Due)
	form.draft.recurrence = cloneRecurrence(req.Recurrence)
	form.titleInput.SetValue(req.Title)
	form.dueTextInput.SetValue(form.draft.dueInput)
	form.presetPlan = req.PlanningArtifacts
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
		m.project = candidate
		m.initCaches()
		m.updatePreview()
		return
	}
}

func (m *TaskComposerModel) View() tea.View {
	if m.mode == ComposerExpanded && m.expanded != nil {
		return m.expanded.View()
	}
	applyThemedTextarea(&m.input)
	var b strings.Builder
	b.WriteString(renderFullscreenTitle("Task Composer", m.preset.label()))
	b.WriteString("\n\n")
	project := "Unavailable"
	if m.preset.Project != nil {
		project = m.preset.Project.Name
	}
	b.WriteString(SubtitleStyle.Render("Project: ") + truncateToVisualWidth(project, max(m.width-12, 1)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Preset: " + m.renderSharedAttributes()))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())

	if count := m.nonblankLineCount(); count > 1 {
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%d tasks · shared attributes apply to every line", count)))
	} else if m.previewErr != "" {
		b.WriteString("\n\n")
		b.WriteString(ValidationErrorStyle.Render(m.previewErr))
	} else if m.preview != nil {
		preview, _, _ := m.composePreviewBlock(max(m.height-10, 2), 0)
		if preview != "" {
			b.WriteString("\n\n")
			b.WriteString(preview)
		}
	}
	if m.isAutocompleteActive() {
		b.WriteString("\n\n")
		b.WriteString(m.ac.RenderDropdownWithLimit("", 5))
	} else if m.helpExpanded {
		m.setSyntaxViewportHeight(max(m.height-12, 3))
		b.WriteString("\n\n")
		b.WriteString(m.syntaxVP.View())
	}
	if len(m.errors) > 0 {
		b.WriteString("\n\n")
		for _, lineErr := range m.errors {
			b.WriteString(ValidationErrorStyle.Render(fmt.Sprintf("Line %d: %s", lineErr.line, lineErr.message)))
			b.WriteString("\n")
		}
	}
	if m.confirmDiscard {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render("Discard input? (y/n)"))
	}
	return tea.NewView(clampFormBody(strings.TrimRight(b.String(), "\n"), max(m.height, 1)))
}

func (m *TaskComposerModel) renderSharedAttributes() string {
	parts := make([]string, 0, 6)
	if state := m.preset.Shared.State; state != "" {
		parts = append(parts, string(state))
	}
	if parent := m.preset.Shared.ParentID; parent != nil {
		parts = append(parts, "parent "+*parent)
	}
	if len(m.preset.Shared.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(m.preset.Shared.Tags, " #"))
	}
	if m.preset.PlanningArtifacts != nil {
		parts = append(parts, "planning membership")
	}
	if len(parts) == 0 {
		return "task · priority 3"
	}
	return strings.Join(parts, " · ")
}

func (m *TaskComposerModel) nonblankLineCount() int {
	count := 0
	for _, line := range strings.Split(m.input.Value(), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func (m *TaskComposerModel) ShouldCaptureEsc() bool { return true }
func (m *TaskComposerModel) HasTextInput() bool     { return true }
func (m *TaskComposerModel) IsFullHeight() bool     { return false }
func (m *TaskComposerModel) StatusBarLabel() string { return "Task Composer" }

// GetConfirmDialog surfaces the embedded form's confirmation in expanded mode
// so the App shell can render and dismiss it. Compact mode owns its own discard
// state directly on the composer.
func (m *TaskComposerModel) GetConfirmDialog() *ConfirmDialog {
	if m.mode == ComposerExpanded && m.expanded != nil {
		return m.expanded.GetConfirmDialog()
	}
	return m.discardConfirmState.GetConfirmDialog()
}

func (m *TaskComposerModel) ContextualFooter() (StatusBarFooterSpec, bool) {
	if m.mode == ComposerExpanded {
		return StatusBarFooterSpec{}, false
	}
	spec := captureFooterSpecFull(
		m.confirmDiscard,
		m.isAutocompleteActive(),
		m.helpExpanded,
		m.helpExpanded && m.syntaxOverflows(),
		m.syntaxScrollPercent(),
	)
	if !m.confirmDiscard && !m.isAutocompleteActive() && !m.helpExpanded {
		spec.Left += " · Shift+Enter new line · Ctrl+F expand"
		if m.preset.AllowProjectSwitch && len(m.preset.AllProjects) > 1 {
			spec.Left += " · Ctrl+P project"
		}
	}
	if spec.Right == "" {
		spec.Right = m.preset.label()
	}
	return spec, true
}

func (m *TaskComposerModel) HandledKeys() []string {
	if m.mode == ComposerExpanded && m.expanded != nil {
		return m.expanded.HandledKeys()
	}
	return []string{"enter", "ctrl+s", "shift+enter", "ctrl+f", "ctrl+p", "ctrl+d", "ctrl+o", "?", "esc"}
}
