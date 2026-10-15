package taskform

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
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
	// HeaderLabel overrides the kind's generic subtitle. Capture names the act
	// and not its destination: the route row under the divider resolves the
	// project on every keystroke, so a label naming one would be the same answer
	// twice and the stale one of the two after Ctrl+P. A capture under a parent
	// still names it here, because that is the one thing the route row does not
	// carry.
	HeaderLabel        string
	Project            *models.Project
	AllProjects        []*models.Project
	Shared             TaskComposerSharedAttributes
	InitialText        string
	AllowProjectSwitch bool
	PreferMultiline    bool
	// Seed opens the composer straight into the expanded form over a prepared
	// draft. It is the typed alternative to InitialText: text goes through the
	// capture grammar and a seed does not, which is what lets a caller start a
	// draft from field values that would otherwise be read as syntax.
	Seed *TaskComposerSeed
}

// TaskComposerSeed is a complete expanded-form draft expressed as plain values.
// Everything in it is copied into the form at construction, so the seed's owner
// may hold no live model pointer and the draft that opens is independent of
// whatever it was derived from.
//
// The checklist is text alone. A seeded checklist starts wholly unticked by
// construction rather than by a reset the caller has to remember, which is the
// only tick state a fresh draft can honestly carry.
type TaskComposerSeed struct {
	// HostTitle names the overlay, replacing the generic expanded heading.
	HostTitle      string
	Title          string
	Description    string
	Priority       int
	Tags           []string
	Context        string
	ChecklistItems []string
	// Destination is the lifecycle the draft starts in. Empty means Backlog:
	// a seeded draft is never Inbox, because a seed is a considered starting
	// point rather than an unsorted one.
	Destination app.CaptureDestination
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
type TaskComposerModel struct {
	pulse.WorkspacePulseState
	grammar.QuickCaptureInput
	Preset   TaskComposerPreset
	runtime  *config.RuntimeState
	Mode     TaskComposerMode
	Expanded *TaskFormModel
	capture  coordinator.CaptureCoordinator
	taskForm coordinator.TaskFormCoordinator
	Errors   []grammar.LineError
	width    int
	height   int
	// shellFrame is what the shell last said its own footer costs. It travels
	// beside the size rather than through it, because the panel must not lose its
	// draft or its preview to a footer that grew a row.
	shellFrame         chrome.ShellFrameMetrics
	SubmissionInFlight bool
	// Once a submit has failed, edits keep its line diagnostics current without
	// moving the caret or issuing another capture request.
	validateOnEdit         bool
	pasteRefusal           string
	batchInvalidLines      int
	batchDestinations      []string
	batchPlanSummary       string
	matchingTaskPreview    *viewport.Model
	matchingTaskText       string
	matchingTaskTexts      []string
	matchingTaskReferences []string
	matchingTaskIndex      int
	// projectOffer is the armed question a refused submit asks when every
	// refusal is a project nobody holds yet; nil when none is asked.
	projectOffer *composerProjectOffer
	// pendingCreatedProjects are the projects an in-flight submit creates, kept
	// so a composer that stays open can route to them once it lands.
	pendingCreatedProjects []*models.Project
	chrome.DiscardConfirmState
}

func NewTaskComposerModel(preset TaskComposerPreset, runtime *config.RuntimeState) *TaskComposerModel {
	m := &TaskComposerModel{
		Preset:  preset,
		runtime: runtime,
		Mode:    ComposerCompact,
		width:   80,
		height:  22,
	}

	m.initCompactInput()
	if preset.Seed != nil {
		m.seedExpandedForm(*preset.Seed)
	}
	return m
}

func (m *TaskComposerModel) initCompactInput() {
	project := m.Preset.Project
	placeholder := "Type a task title…"
	if m.Preset.PreferMultiline {
		placeholder = "One task per line…"
	}
	m.QuickCaptureInput = grammar.NewQuickCaptureInput(project, m.Preset.AllProjects, placeholder, !m.Preset.AllowProjectSwitch)
	m.DefaultParentID = cloneStringPtr(m.Preset.Shared.ParentID)
	m.HostRouteSummary = m.routeSummary
	m.Input.CharLimit = 4000
	if m.Preset.InitialText != "" {
		m.Input.SetValue(m.Preset.InitialText)
		m.Input.CursorEnd()
		m.UpdatePreview()
	}
	if project == nil {
		m.Errors = []grammar.LineError{{Line: 1, Message: "composer preset requires a project"}}
	}
	// Last, because the height the input opens at is what this preset's own
	// draft and refusals ask for: a seeded line and a preset with no project
	// both open at the size they will be rendered at rather than at a literal
	// the first frame would have to correct.
	m.refreshBatchDiagnostics()
	m.syncInputHeight()
}

func (m *TaskComposerModel) SetCaptureCoordinator(c coordinator.CaptureCoordinator) { m.capture = c }

func (m *TaskComposerModel) SetTaskFormCoordinator(c coordinator.TaskFormCoordinator) {
	m.taskForm = c
	if m.Expanded != nil {
		m.Expanded.SetTaskFormCoordinator(c)
	}
}

func (m *TaskComposerModel) SetSize(width, height int) {
	m.width, m.height = width, height
	if m.Expanded != nil {
		m.Expanded.SetSize(width, height)
	}
	if m.matchingTaskPreview != nil {
		m.resizeMatchingTaskPreview()
	}
	if m.Mode == ComposerCompact {
		m.Width = max(width-2, 1)
		m.Input.SetWidth(max(min(width-4, 90), 20))
		m.SetSyntaxViewportWidth(max(width-2, 1))
		// After the width, since the width is what the draft wraps at and the
		// wrapping is what decides how many rows it needs.
		m.syncInputHeight()
	}
}

func (m *TaskComposerModel) SetWorkspacePulse(workspacePulse pulse.WorkspacePulse) {
	m.WorkspacePulseState.SetWorkspacePulse(workspacePulse)
	if m.Expanded != nil {
		m.Expanded.SetWorkspacePulse(workspacePulse)
	}
}

// HasDraft reports whether the compact buffer holds work Esc would have to ask
// about. The key handler and the footer read this one predicate, so the hint and
// the act it names cannot drift apart.
func (m *TaskComposerModel) HasDraft() bool {
	return strings.TrimSpace(m.Input.Value()) != ""
}

func (m *TaskComposerModel) Init() tea.Cmd {
	if m.Expanded != nil {
		return m.Expanded.Init()
	}
	return textarea.Blink
}

// updateSubmission releases the pending save only on a result. While it is
// pending, input stays frozen so success cannot erase text typed after submit.
func (m *TaskComposerModel) updateSubmission(msg tea.Msg) bool {
	switch result := msg.(type) {
	case coordinator.TaskCreatedMsg:
		if m.SubmissionInFlight && result.KeepComposerOpen {
			m.Input.SetValue("")
			m.Errors = nil
			m.validateOnEdit = false
			m.batchInvalidLines = 0
			m.UpdatePreview()
			m.syncInputHeight()
			m.adoptCreatedProjects()
		}
		m.pendingCreatedProjects = nil
		m.SubmissionInFlight = false
	case coordinator.TasksCapturedMsg, status.ErrorMsg:
		m.pendingCreatedProjects = nil
		m.SubmissionInFlight = false
	}
	if m.SubmissionInFlight {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.PasteMsg:
			return true
		}
	}

	return false
}

func (m *TaskComposerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.updateSubmission(msg) {
		return m, nil
	}

	if m.Mode == ComposerExpanded {
		if m.Expanded == nil {
			return m, nil
		}
		updated, cmd := m.Expanded.Update(msg)
		if form, ok := updated.(*TaskFormModel); ok {
			m.Expanded = form
		}
		return m, cmd
	}

	if m.matchingTaskPreview != nil {
		return m, m.updateMatchingTaskPreview(msg)
	}

	if m.projectOffer != nil {
		return m.updateProjectOffer(msg)
	}

	if m.ConfirmDiscard {
		return m.updateDiscardConfirmation(msg)
	}

	defer m.refreshCaptureErrorsAfterEdit(m.Input.Value(), m.Preset.Project)

	if key, ok := msg.(tea.KeyPressMsg); ok {
		name := key.String()
		if handled, cmd := m.handleComposerAutocompleteKey(name); handled {
			return m, cmd
		}
		if handled, cmd := m.handleCaptureUtilityKey(name); handled {
			return m, cmd
		}
		if m.HelpExpanded && !m.IsAutocompleteActive() {
			switch name {
			case "ctrl+k":
				m.SyntaxScrollLine(-1)
				return m, nil
			case "ctrl+j":
				m.SyntaxScrollLine(1)
				return m, nil
			case "pgup":
				m.SyntaxScrollPage(-1)
				return m, nil
			case "pgdown":
				m.SyntaxScrollPage(1)
				return m, nil
			}
		}
		if name == "?" && m.AC.Mode == grammar.ACModeNone {
			m.ToggleHelp()
			return m, nil
		}
		switch name {
		case "esc":
			if m.HelpExpanded {
				m.CloseHelp()
				return m, nil
			}
			if m.HasDraft() {
				m.ConfirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return status.BackMsg{} }
		case "shift+enter":
			m.Input.InsertString("\n")
			m.UpdatePreview()
			return m, nil
		case "enter", "ctrl+s":
			if m.AC.Mode == grammar.ACModeNone {
				return m, m.CreateCompact(false)
			}
		case "ctrl+e":
			if m.AC.Mode == grammar.ACModeNone {
				return m, m.CreateCompact(true)
			}
		case "ctrl+f":
			return m, m.ExpandSingle()
		case "ctrl+p":
			m.cycleProject()
			return m, nil
		}
	}

	return m, m.updateCompactInput(msg)
}

func (m *TaskComposerModel) updateDiscardConfirmation(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "y", "Y":
			m.ConfirmDiscard = false
			return m, func() tea.Msg { return status.BackMsg{} }
		case "n", "N", "esc":
			m.ConfirmDiscard = false
		}
	}
	return m, nil
}

func (m *TaskComposerModel) handleCaptureUtilityKey(name string) (bool, tea.Cmd) {
	if name != "ctrl+n" && name != "alt+n" && name != "alt+p" && name != "ctrl+o" {
		return false, nil
	}
	if m.IsAutocompleteActive() || m.HelpExpanded {
		return true, nil
	}
	if name == "ctrl+o" {
		m.openMatchingTaskPreview()
		return true, nil
	}
	if name == "ctrl+n" {
		return true, m.createCompact(false, true)
	}
	if name == "alt+p" {
		m.focusPreviousCaptureError()
	} else {
		m.focusNextCaptureError()
	}
	return true, nil
}

func (m *TaskComposerModel) refreshCaptureErrorsAfterEdit(before string, projectBefore *models.Project) {
	if m.Mode == ComposerCompact &&
		(m.Input.Value() != before || m.Preset.Project != projectBefore) {
		m.pasteRefusal = ""
		m.refreshBatchDiagnostics()
		if m.validateOnEdit {
			m.ParseCompactLines()
		}
		m.syncInputHeight()
	}
}

func (m *TaskComposerModel) handleComposerAutocompleteKey(name string) (bool, tea.Cmd) {
	if name == "enter" && m.AutocompleteLiteralComplete() {
		return true, m.CreateCompact(false)
	}
	if m.HandleAutocompleteKey(name) {
		return true, nil
	}
	return false, nil
}

func (m *TaskComposerModel) CreateCompact(openContent bool) tea.Cmd {
	return m.createCompact(openContent, false)
}

func (m *TaskComposerModel) createCompact(openContent, keepOpen bool) tea.Cmd {
	if m.SubmissionInFlight {
		return nil
	}
	m.pasteRefusal = ""
	reqs, ok := m.ParseCompactLines()
	if !ok {
		m.validateOnEdit = true
		m.focusFirstCaptureError()
		m.offerProjectCreation(openContent, keepOpen)
		return nil
	}
	var cmd tea.Cmd
	if len(reqs) == 1 {
		cmd = m.capture.CaptureCreate(reqs[0], openContent)
	} else {
		cmd = m.capture.CaptureBulk(reqs, true)
	}
	if cmd == nil {
		return nil
	}
	return m.markSubmission(cmd, keepOpen)
}

// markSubmission freezes input until the capture's result arrives and, for
// Ctrl+N, marks a success as one that keeps the composer open.
func (m *TaskComposerModel) markSubmission(cmd tea.Cmd, keepOpen bool) tea.Cmd {
	m.SubmissionInFlight = true
	if !keepOpen {
		return cmd
	}
	return func() tea.Msg {
		result := cmd()
		if created, ok := result.(coordinator.TaskCreatedMsg); ok {
			created.KeepComposerOpen = true
			return created
		}
		return result
	}
}

// focusFirstCaptureError runs only after an explicit submit. Walking visual
// rows also handles wrapped lines and scrolls an off-screen error into view.
func (m *TaskComposerModel) focusFirstCaptureError() {
	if len(m.Errors) == 0 {
		return
	}
	m.focusCaptureErrorLine(m.Errors[0].Line)
}

func (m *TaskComposerModel) focusNextCaptureError() {
	if len(m.Errors) == 0 {
		return
	}
	target := m.Errors[0].Line
	for _, problem := range m.Errors {
		if problem.Line > m.Input.Line()+1 {
			target = problem.Line
			break
		}
	}
	m.focusCaptureErrorLine(target)
}

func (m *TaskComposerModel) focusPreviousCaptureError() {
	if len(m.Errors) == 0 {
		return
	}
	target := m.Errors[len(m.Errors)-1].Line
	for i := len(m.Errors) - 1; i >= 0; i-- {
		if m.Errors[i].Line < m.Input.Line()+1 {
			target = m.Errors[i].Line
			break
		}
	}
	m.focusCaptureErrorLine(target)
}

func (m *TaskComposerModel) focusCaptureErrorLine(line int) {
	m.syncInputHeight()
	target := max(line-1, 0)
	m.Input.MoveToBegin()
	for guard := m.composerInputContentRows(); m.Input.Line() < target && guard > 0; guard-- {
		m.Input.CursorDown()
	}
	column := 0
	for _, problem := range m.Errors {
		if problem.Line == line {
			column = problem.Column
			break
		}
	}
	m.Input.SetCursorColumn(column)
	// Setting a column alone does not scroll a wrapped token into view.
	m.Input, _ = m.Input.Update(nil)
	m.UpdatePreview()
}

func (m *TaskComposerModel) ParseCompactLines() ([]app.CaptureRequest, bool) {
	reqs, lineErrors := m.parseCompactLines()
	m.Errors = lineErrors
	return reqs, len(lineErrors) == 0
}

// Live batch feedback uses the submit parser without publishing refusals or
// moving the caret. Rendering and layout only read the settled count.
func (m *TaskComposerModel) refreshBatchDiagnostics() {
	m.batchInvalidLines = 0
	m.batchDestinations = nil
	m.batchPlanSummary = ""
	if m.nonblankLineCount() > 1 {
		requests, lineErrors := m.parseCompactLines()
		m.batchInvalidLines = len(lineErrors)
		counts := make(map[string]int)
		planCounts := make(map[app.CaptureDestination]int)
		var order []string
		for _, request := range requests {
			if counts[request.ProjectID] == 0 {
				order = append(order, request.ProjectID)
			}
			counts[request.ProjectID]++
			planCounts[composerPlanDestination(request.Destination)]++
		}
		for _, id := range order {
			project := m.projectByID(id)
			name := project.Slug
			if name == "" {
				name = project.Name
			}
			m.batchDestinations = append(m.batchDestinations, fmt.Sprintf("%s %d", name, counts[id]))
		}
		if len(planCounts) > 1 {
			var parts []string
			for _, plan := range []app.CaptureDestination{app.CaptureDestinationToday, app.CaptureDestinationNext, ""} {
				if count := planCounts[plan]; count > 0 {
					label := "Unplanned"
					if plan != "" {
						label = chrome.PlanStatusValue(string(plan))
					}
					parts = append(parts, fmt.Sprintf("%s %d", label, count))
				}
			}
			m.batchPlanSummary = "Plan: " + strings.Join(parts, " · ")
		}
	}
}

func (m *TaskComposerModel) parseCompactLines() ([]app.CaptureRequest, []grammar.LineError) {
	return capturetext.Requests(m.Input.Value(), m.Preset.Project, m.Preset.AllProjects, m.captureDefaults())
}

// captureDefaults is the preset's half of every line: the shared values a line's
// own tokens override, read by the one request builder the bridge also uses.
func (m *TaskComposerModel) captureDefaults() capturetext.Defaults {
	return capturetext.Defaults{
		Priority:    m.Preset.Shared.Priority,
		Tags:        m.Preset.Shared.Tags,
		ParentID:    m.Preset.Shared.ParentID,
		Due:         m.Preset.Shared.Due,
		Destination: m.Preset.Shared.Destination,
		SkipProject: !m.Preset.AllowProjectSwitch,
	}
}

// ParseCaptureText reads text exactly as the compact composer would on
// submit — every nonblank line through the capture grammar and the preset's
// defaults — without a composer on screen. A host capturing through the
// control socket uses it, so a line refused here is one `a` would refuse too.
func ParseCaptureText(preset TaskComposerPreset, runtime *config.RuntimeState, text string) ([]app.CaptureRequest, []grammar.LineError) {
	composer := NewTaskComposerModel(preset, runtime)
	composer.Input.SetValue(text)
	reqs, ok := composer.ParseCompactLines()
	if !ok {
		return nil, composer.Errors
	}
	return reqs, nil
}

// CapturePreviewText describes a host's full draft through the composer's own
// validation and route summary. Invalid drafts return no misleading route;
// opening an empty field still describes the default destination.
func CapturePreviewText(preset TaskComposerPreset, runtime *config.RuntimeState, text string) (string, []grammar.LineError) {
	composer := NewTaskComposerModel(preset, runtime)
	composer.Input.SetValue(text)
	var reqs []app.CaptureRequest
	if strings.TrimSpace(text) != "" {
		var ok bool
		reqs, ok = composer.ParseCompactLines()
		if !ok {
			return "", composer.Errors
		}
	}
	project, plan, state := composer.effectiveRouteHeader().strings()
	parts := []string{"Project: " + project}
	if plan != "" {
		parts = append(parts, "Plan: "+plan)
	}
	parts = append(parts, "Status: "+state)
	now := time.Now()
	var due string
	for i, req := range reqs {
		value := ""
		if req.Due != nil {
			value = chrome.FormatWeekdayDate(*req.Due, now, (&models.Task{DueDate: req.Due}).DueHasTime())
		}
		if i == 0 {
			due = value
		} else if due != value {
			due = composerMixedValue
		}
	}
	if due != "" {
		parts = append(parts, "Due: "+due)
	}
	for _, field := range []struct {
		label string
		value func(app.CaptureRequest) string
	}{
		{"Defer until", func(req app.CaptureRequest) string {
			if req.Defer == nil {
				return ""
			}
			return chrome.FormatWeekdayDate(req.Defer.Local(), now, true)
		}},
		{"Priority", func(req app.CaptureRequest) string {
			if req.Priority == models.PriorityUnset {
				return ""
			}
			return models.PriorityName(req.Priority)
		}},
	} {
		value := ""
		for i, req := range reqs {
			next := field.value(req)
			if i == 0 {
				value = next
			} else if next != value {
				value = composerMixedValue
			}
		}
		if value != "" {
			parts = append(parts, field.label+": "+value)
		}
	}
	var notices []string
	for _, req := range reqs {
		project := composer.projectByID(req.ProjectID)
		input := grammar.QuickCaptureInput{Project: project,
			Preview: &grammar.QuickCaptureResult{Title: req.Title, ParentID: req.ParentID}}
		if hint := input.DuplicateCaptureHint(); hint != "" {
			if len(reqs) > 1 {
				hint = fmt.Sprintf("Line %d: %s", req.SourceLine, hint)
			}
			notices = append(notices, hint)
		}
	}
	return strings.Join(append([]string{strings.Join(parts, " · ")}, notices...), "\n"), nil
}

// CaptureCompletion is one suggestion CaptureCompletions offers: Label is the
// plain text a list shows and Insert the whole replacement for the token.
type CaptureCompletion struct {
	Label  string
	Insert string
}

// CaptureCompletionSet is what the compact composer would offer at a caret.
// Token is the text being completed, sigil included, and empty when the caret
// stands on no sigil the grammar completes. Exact is the composer's own test
// for whether Enter submits instead of accepting.
type CaptureCompletionSet struct {
	Token string
	Items []CaptureCompletion
	Exact bool
}

// captureCompletionWidth is the width a headless composer truncates its task
// titles at. A host draws its own list, so the budget is generous rather than a
// terminal's, while still bounding a pathological title.
const captureCompletionWidth = 120

// CaptureCompletions reads text as the draft up to the caret and returns what
// the compact composer would suggest there, each suggestion as accepting it
// would rewrite the token. A host completing through the control socket uses
// it, so it offers exactly what `a` offers.
func CaptureCompletions(preset TaskComposerPreset, runtime *config.RuntimeState, text string) CaptureCompletionSet {
	composer := NewTaskComposerModel(preset, runtime)
	composer.SetSize(captureCompletionWidth, composer.height)
	composer.Input.SetValue(text)
	composer.RefreshAutocomplete()
	value := composer.Input.Value()
	if !composer.AC.IsActive() || composer.AC.TriggerPos > len(value) {
		return CaptureCompletionSet{}
	}
	trigger := grammar.TriggerCharForMode(composer.AC.Mode, composer.Triggers)
	set := CaptureCompletionSet{
		Token: value[composer.AC.TriggerPos:],
		Items: make([]CaptureCompletion, 0, len(composer.AC.Items)),
		Exact: composer.AutocompleteLiteralComplete(),
	}
	for _, item := range composer.AC.Items {
		set.Items = append(set.Items, CaptureCompletion{
			Label:  ansi.Strip(item.Display),
			Insert: trigger + item.Value + " ",
		})
	}
	return set
}
