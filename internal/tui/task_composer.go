package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	ComposerChild
)

type TaskComposerMode uint8

const (
	ComposerCompact TaskComposerMode = iota
	ComposerExpanded
)

// TaskComposerSharedAttributes are defaults shared by every nonblank compact
// line. Explicit capture syntax on a line wins over these defaults.
type TaskComposerSharedAttributes struct {
	Priority    int
	Tags        []string
	Due         *time.Time
	ParentID    *string
	Destination app.CaptureDestination
}

// TaskComposerPreset is the complete typed launch contract for compact create,
// expanded create/edit, children, and links.
type TaskComposerPreset struct {
	Kind TaskComposerPresetKind
	// HeaderLabel overrides the kind's generic subtitle. Capture uses it to name
	// the destination project it resolved, because base capture and header-row
	// capture reach the same overlay with the same kind and would otherwise be
	// indistinguishable until the task landed somewhere unexpected.
	HeaderLabel        string
	Project            *models.Project
	AllProjects        []*models.Project
	Shared             TaskComposerSharedAttributes
	InitialText        string
	AllowProjectSwitch bool
	PreferMultiline    bool
}

func (p TaskComposerPreset) label() string {
	if p.HeaderLabel != "" {
		return p.HeaderLabel
	}
	switch p.Kind {
	case ComposerGlobal:
		return "Global capture"
	case ComposerProject:
		return "Project task"
	case ComposerChild:
		return "Child task"
	default:
		return "Task"
	}
}

func (p TaskComposerPreset) defaultDestination(
	parentID *string,
) app.CaptureDestination {
	if parentID != nil {
		return app.CaptureDestinationBacklog
	}
	if p.Shared.Destination != "" {
		return p.Shared.Destination
	}
	return app.CaptureDestinationInbox
}

// TaskComposerModel is the sole live task creation overlay. Compact mode owns
// one-or-many capture lines; expanded mode embeds the full form for one task.
type lineError struct {
	line    int
	message string
}

type TaskComposerModel struct {
	workspacePulseState
	quickCaptureInput
	preset             TaskComposerPreset
	runtime            *config.RuntimeState
	mode               TaskComposerMode
	expanded           *TaskFormModel
	capture            CaptureCoordinator
	taskForm           TaskFormCoordinator
	errors             []lineError
	width              int
	height             int
	submissionInFlight bool
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

	m.initCompactInput()
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
	m.input.SetHeight(3)
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
		m.input.SetHeight(3)
		m.setSyntaxViewportWidth(max(width-2, 1))
	}
}

func (m *TaskComposerModel) setWorkspacePulse(pulse WorkspacePulse) {
	m.workspacePulseState.setWorkspacePulse(pulse)
	if m.expanded != nil {
		m.expanded.setWorkspacePulse(pulse)
	}
}

func (m *TaskComposerModel) Init() tea.Cmd {
	if m.expanded != nil {
		return m.expanded.Init()
	}
	return textarea.Blink
}

func (m *TaskComposerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case TaskCreatedMsg, ErrorMsg:
		m.submissionInFlight = false
	}

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
		if handled, cmd := m.handleComposerAutocompleteKey(name); handled {
			return m, cmd
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
				return m, m.createCompact(false)
			}
		case "ctrl+e":
			if m.ac.Mode == acModeNone {
				return m, m.createCompact(true)
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

func (m *TaskComposerModel) handleComposerAutocompleteKey(name string) (bool, tea.Cmd) {
	if name == "enter" && m.autocompleteLiteralComplete() {
		return true, m.createCompact(false)
	}
	if m.handleAutocompleteKey(name) {
		return true, nil
	}
	return false, nil
}

func (m *TaskComposerModel) createCompact(openContent bool) tea.Cmd {
	if m.submissionInFlight {
		return nil
	}
	reqs, ok := m.parseCompactLines()
	if !ok {
		return nil
	}
	var cmd tea.Cmd
	if len(reqs) == 1 {
		cmd = m.capture.CaptureCreate(reqs[0], openContent)
	} else {
		cmd = m.capture.CaptureBulk(reqs, true)
	}
	if cmd != nil {
		m.submissionInFlight = true
	}
	return cmd
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
		destination := parsed.destination
		if !parsed.destinationSet {
			destination = m.preset.defaultDestination(parentID)
		}
		if dueDateIsToday(due) {
			destination = app.CaptureDestinationToday
		}
		req := app.CaptureRequest{
			SourceLine:  idx + 1,
			ProjectID:   target.ID,
			Title:       parsed.title,
			Priority:    priority,
			Tags:        mergeComposerTags(m.preset.Shared.Tags, parsed.tags),
			Due:         due,
			ParentID:    parentID,
			Destination: destination,
			Prepend:     true,
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
	destination := req.Destination
	if destination == "" {
		destination = m.preset.defaultDestination(req.ParentID)
	}
	form.draft.state = destination.WorkflowState()
	form.draft.parentID = cloneStringPtr(req.ParentID)
	form.draft.dueDate = cloneTime(req.Due)
	form.draft.dueInput = formatDateInput(req.Due)
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
		m.project = candidate
		m.initCaches()
		m.updatePreview()
		return
	}
}

type composerEffectiveRoute struct {
	projectID   string
	projectName string
	destination app.CaptureDestination
}

func (m *TaskComposerModel) effectiveRouteForLine(line string) composerEffectiveRoute {
	project := m.preset.Project
	parsed, _ := parseQuickCaptureForPreview(
		strings.TrimSpace(line),
		m.preset.Project,
		m.preset.AllProjects,
		!m.preset.AllowProjectSwitch,
	)
	if parsed != nil && parsed.targetProjectID != nil {
		if target := m.projectByID(*parsed.targetProjectID); target != nil {
			project = target
		}
	}

	var parentID *string
	if parsed != nil {
		parentID = parsed.parentID
	}
	if parentID == nil {
		parentID = m.preset.Shared.ParentID
	}
	destination := m.preset.defaultDestination(parentID)
	if parsed != nil && parsed.destinationSet {
		destination = parsed.destination
	}
	var due *time.Time
	if parsed != nil {
		due = parsed.dueDate
	}
	if due == nil {
		due = m.preset.Shared.Due
	}
	if dueDateIsToday(due) {
		destination = app.CaptureDestinationToday
	}

	route := composerEffectiveRoute{destination: destination}
	if project != nil {
		route.projectID = project.ID
		route.projectName = project.Name
	} else {
		route.projectName = "Unavailable"
	}
	return route
}

func (m *TaskComposerModel) effectiveRouteHeader() (project, destination string) {
	var routes []composerEffectiveRoute
	for _, line := range strings.Split(m.input.Value(), "\n") {
		if strings.TrimSpace(line) != "" {
			routes = append(routes, m.effectiveRouteForLine(line))
		}
	}
	if len(routes) == 0 {
		routes = append(routes, m.effectiveRouteForLine(""))
	}

	project = routes[0].projectName
	destination = renderComposerDestination(routes[0].destination)
	for _, route := range routes[1:] {
		if route.projectID != routes[0].projectID {
			project = "Mixed"
		}
		if route.destination != routes[0].destination {
			destination = "Mixed"
		}
	}
	return project, destination
}

func renderComposerDestination(destination app.CaptureDestination) string {
	if destination == "" {
		destination = app.CaptureDestinationInbox
	}
	return destination.WorkflowState().Icon() + " $" + string(destination)
}

// compactContentHeight converts the shell's conventional height-minus-two view
// budget into the rows actually available above this composer's contextual
// footer. The footer can wrap beyond its usual divider-plus-one-row shape at
// narrower widths, and every extra footer row must be removed from the lower
// pane budget before the shell performs its final safety clip.
func (m *TaskComposerModel) compactContentHeight() int {
	contentHeight := m.height
	if spec, ok := m.ContextualGuidance(); ok {
		footerRows := lipgloss.Height(renderGuidance(spec, m.width))
		contentHeight -= max(footerRows-2, 0)
	}
	return max(contentHeight, 1)
}

func (m *TaskComposerModel) View() tea.View {
	if m.mode == ComposerExpanded && m.expanded != nil {
		return m.expanded.View()
	}
	applyThemedTextarea(&m.input)
	contentHeight := m.compactContentHeight()
	project, destination := m.effectiveRouteHeader()
	var b strings.Builder
	b.WriteString(renderFullscreenTitleWithPulse("Task Composer", m.preset.label(), m.currentWorkspacePulse(), m.width))
	b.WriteString("\n\n")
	b.WriteString(SubtitleStyle.Render("Project: ") + truncateToVisualWidth(project, max(m.width-12, 1)))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Destination: ") + truncateToVisualWidth(destination, max(m.width-16, 1)))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(renderHorizontalDivider(m.width))

	var errorsBlock strings.Builder
	for _, lineErr := range m.errors {
		errorsBlock.WriteString(ValidationErrorStyle.Render(fmt.Sprintf("Line %d: %s", lineErr.line, lineErr.message)))
		errorsBlock.WriteString("\n")
	}
	errors := strings.TrimRight(errorsBlock.String(), "\n")
	confirm := ""
	if m.confirmDiscard {
		confirm = ValidationErrorStyle.Render("Discard input? (y/n)")
	}

	blockRows := func(blocks ...string) int {
		rows := 0
		for _, block := range blocks {
			if block != "" {
				rows += 1 + lipgloss.Height(block)
			}
		}
		return rows
	}
	appendBlock := func(block string) {
		if block == "" {
			return
		}
		b.WriteString("\n\n")
		b.WriteString(block)
	}

	prefixRows := lipgloss.Height(b.String())
	remaining := max(contentHeight-prefixRows-blockRows(errors, confirm), 1)
	preview := ""
	if count := m.nonblankLineCount(); count > 1 {
		preview = MutedStyle.Render(fmt.Sprintf("%d tasks · shared attributes apply to every line", count))
	} else if m.previewErr != "" || m.preview != nil {
		lowerMin := 0
		if m.isAutocompleteActive() {
			lowerMin = min(len(m.ac.Items), 3)
		}
		preview, _, _ = m.composePreviewBlock(max(remaining-1, 1), lowerMin)
	}

	appendBlock(preview)
	fixedRows := prefixRows + blockRows(preview, errors, confirm)
	if m.isAutocompleteActive() {
		rowBudget := max(contentHeight-fixedRows-1, 1)
		appendBlock(strings.TrimRight(m.ac.RenderDropdownWithLimit("", rowBudget), "\n"))
	} else if m.helpExpanded {
		helpBudget := max(contentHeight-fixedRows-1, 1)
		m.setSyntaxViewportHeight(helpBudget)
		appendBlock(m.syntaxVP.View())
	}
	appendBlock(errors)
	appendBlock(confirm)
	return tea.NewView(clampFormBody(strings.TrimRight(b.String(), "\n"), contentHeight))
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

func (m *TaskComposerModel) ContextualGuidance() (GuidanceSpec, bool) {
	if m.mode == ComposerExpanded {
		return GuidanceSpec{}, false
	}
	spec := captureGuidanceSpec(
		m.confirmDiscard,
		m.isAutocompleteActive(),
		m.helpExpanded,
		m.helpExpanded && m.syntaxOverflows(),
		m.syntaxScrollPercent(),
	)
	if len(m.errors) > 0 && !m.confirmDiscard && !m.isAutocompleteActive() && !m.helpExpanded {
		spec = requiredActionGuidanceSpec(
			"Enter retry", "Ctrl+E retry & edit", "Ctrl+V paste", "? syntax",
		)
	}
	if !m.confirmDiscard && !m.isAutocompleteActive() && !m.helpExpanded {
		spec.Hints = append(spec.Hints, optionalGuidanceHints(100,
			"Shift+Enter new line", "Ctrl+F expand",
		)...)
		if m.preset.AllowProjectSwitch && len(m.preset.AllProjects) > 1 {
			spec.Hints = append(spec.Hints, optionalGuidanceHints(102, "Ctrl+P project")...)
		}
	}
	if spec.Supplemental == "" {
		spec.Supplemental = m.preset.label()
	}
	return spec, true
}

func (m *TaskComposerModel) HandledKeys() []string {
	if m.mode == ComposerExpanded && m.expanded != nil {
		return m.expanded.HandledKeys()
	}
	return []string{"enter", "ctrl+s", "shift+enter", "ctrl+f", "ctrl+p", "ctrl+e", "?", "esc"}
}
