package taskform

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
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
	// Now is the instant a parsed due date is measured against for the Today
	// route; the zero value reads the wall clock.
	Now time.Time
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
	preservesDraft           bool
	recovery                 *config.LocalDraft
	recoveryBlocked          error
	recoveryPending          bool
	recoveryAutosaveSequence uint64
	Preset                   TaskComposerPreset
	runtime                  *config.RuntimeState
	Mode                     TaskComposerMode
	Expanded                 *TaskFormModel
	capture                  coordinator.CaptureCoordinator
	taskForm                 coordinator.TaskFormCoordinator
	Errors                   []grammar.LineError
	width                    int
	height                   int
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
		previousWidth := m.Input.Width()
		m.Input.SetWidth(max(min(width-4, 90), 20))
		if m.Input.Width() != previousWidth {
			// Refresh the viewport's wrapped content before scrolling: its old
			// row count can otherwise clamp the caret above a newly wrapped end.
			_ = m.Input.View()
		}
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

// HasDraft reports whether the compact buffer holds work to preserve on close.
// The key handler and footer share this predicate.
func (m *TaskComposerModel) HasDraft() bool {
	return strings.TrimSpace(m.Input.Value()) != ""
}

func (m *TaskComposerModel) Init() tea.Cmd {
	if m.Expanded != nil {
		return m.Expanded.Init()
	}
	return textarea.Blink
}

// QuitBlockReason names content a desktop quit would abandon.
func (m *TaskComposerModel) QuitBlockReason() string {
	if m.SubmissionInFlight {
		return "A capture is still being saved."
	}
	if m.Mode == ComposerExpanded && m.Expanded != nil {
		return m.Expanded.QuitBlockReason()
	}
	if m.HasDraft() {
		return "A capture draft is open; close it to keep the latest edits."
	}
	return ""
}

func (m *TaskComposerModel) closeCompactDraft() tea.Cmd {
	if m.preservesDraft {
		return m.closeKeepingDraft()
	}
	if m.HasDraft() {
		m.ConfirmDiscard = true
		return nil
	}
	return func() tea.Msg { return status.BackMsg{} }
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
	if autosave, ok := msg.(composerAutosaveMsg); ok {
		m.flushDraftAutosave(autosave)
		return m, nil
	}
	before, enabled := m.autosaveBeforeUpdate()
	model, cmd := m.update(msg)
	if enabled {
		cmd = m.autosaveAfterUpdate(before, cmd)
	}
	return model, cmd
}

// The form has many early returns, including results from its pickers and
// external editor. Update compares the complete recovery value around them so
// every edit gets the same recovery behavior without a second field router.
func (m *TaskComposerModel) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return m, m.closeCompactDraft()
		case "ctrl+x":
			m.ConfirmDiscard = true
			return m, nil
		case "shift+enter", "alt+enter":
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
			if m.preservesDraft {
				return m, m.discardRecovery()
			}
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
