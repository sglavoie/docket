package taskform

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/theme"
)

type composerEffectiveRoute struct {
	project     *models.Project
	destination app.CaptureDestination
}

func (m *TaskComposerModel) effectiveRouteForLine(line string) composerEffectiveRoute {
	project := m.Preset.Project
	parsed, _ := grammar.ParseQuickCaptureForPreview(
		strings.TrimSpace(line),
		m.Preset.Project,
		m.Preset.AllProjects,
		!m.Preset.AllowProjectSwitch,
	)
	if parsed != nil && parsed.TargetProjectID != nil {
		if target := m.projectByID(*parsed.TargetProjectID); target != nil {
			project = target
		}
	}

	var parentID *string
	if parsed != nil {
		parentID = parsed.ParentID
	}
	if parentID == nil {
		parentID = m.Preset.Shared.ParentID
	}
	destination := m.Preset.defaultDestination(parentID)
	if parsed != nil && parsed.DestinationSet {
		destination = parsed.Destination
	}
	var due *time.Time
	if parsed != nil {
		due = parsed.DueDate
	}
	if due == nil {
		due = m.Preset.Shared.Due
	}
	if coordinator.DueDateIsToday(due) {
		destination = app.CaptureDestinationToday
	}
	destination = grammar.ComposerDeferredDestination(parsed, destination)

	return composerEffectiveRoute{project: project, destination: destination}
}

// composerMixedValue is what one axis reports when the lines being captured do
// not agree about it. Each axis answers for itself, so a batch heading for Today
// and for Done is mixed about its status and mixed about its plan, while one
// heading for Today and for Next agrees about status and not about plan.
const composerMixedValue = "Mixed"

// composerUnavailableValue names a route whose project no longer resolves.
const composerUnavailableValue = "Unavailable"

// composerRouteHeader is the route as typed values, so the row can color each
// axis through the authority that owns it rather than recognizing a word. A
// mixed axis carries no value; an absent plan is the zero destination.
type composerRouteHeader struct {
	project      *models.Project
	projectMixed bool
	plan         app.CaptureDestination
	planMixed    bool
	status       models.WorkflowState
	statusMixed  bool
}

// effectiveRouteHeader states the route as the two independent things it is: the
// planning collection the capture commits to, and the lifecycle state it lands
// in. One destination decides both — `$today` is a plan with a Backlog status —
// so a single value naming only the sigil left the second half unsaid.
//
// An empty plan is the absence of a membership rather than an unknown one, and
// the row drops that half entirely; a status is never absent, since every task
// created is in some state.
func (m *TaskComposerModel) effectiveRouteHeader() composerRouteHeader {
	var routes []composerEffectiveRoute
	for _, line := range strings.Split(m.Input.Value(), "\n") {
		if strings.TrimSpace(line) != "" {
			routes = append(routes, m.effectiveRouteForLine(line))
		}
	}
	if len(routes) == 0 {
		routes = append(routes, m.effectiveRouteForLine(""))
	}

	header := composerRouteHeader{
		project: routes[0].project,
		plan:    composerPlanDestination(routes[0].destination),
		status:  composerStatusState(routes[0].destination),
	}
	for _, route := range routes[1:] {
		if composerProjectID(route.project) != composerProjectID(header.project) {
			header.projectMixed = true
		}
		if composerPlanDestination(route.destination) != header.plan {
			header.planMixed = true
		}
		if composerStatusState(route.destination) != header.status {
			header.statusMixed = true
		}
	}
	return header
}

func composerProjectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	return project.ID
}

// strings spells the header uncolored: the project name, the plan, and the
// status, with an absent plan as the empty string.
func (h composerRouteHeader) strings() (project, plan, status string) {
	switch {
	case h.projectMixed:
		project = composerMixedValue
	case h.project == nil:
		project = composerUnavailableValue
	default:
		project = h.project.Name
	}
	switch {
	case h.planMixed:
		plan = composerMixedValue
	case h.plan != "":
		plan = chrome.PlanStatusValue(string(h.plan))
	}
	status = composerMixedValue
	if !h.statusMixed {
		status = chrome.PlanStatusValue(h.status.String())
	}
	return project, plan, status
}

// projectStyle is the project's own color, muted when no one project answers.
func (h composerRouteHeader) projectStyle() lipgloss.Style {
	if h.projectMixed || h.project == nil {
		return theme.MutedStyle
	}
	return theme.ProjectColorStyle(h.project)
}

// planStyle is the plan collection's inline badge, muted when the lines disagree.
func (h composerRouteHeader) planStyle() lipgloss.Style {
	if h.planMixed {
		return theme.MutedStyle
	}
	return grammar.CapturePlanAccentStyle(h.plan)
}

// statusStyle is the lifecycle state's inline badge, muted when the lines disagree.
func (h composerRouteHeader) statusStyle() lipgloss.Style {
	if h.statusMixed {
		return theme.MutedStyle
	}
	return grammar.CaptureStatusAccentStyle(h.status)
}

// composerPlanDestination is the planning collection the effective route
// commits to and nothing at all for the eight destinations that name a state
// alone.
func composerPlanDestination(destination app.CaptureDestination) app.CaptureDestination {
	if !destination.IsPlanning() {
		return ""
	}
	return destination
}

// composerStatusState is the lifecycle state the effective route lands in.
// Today and Next resolve through WorkflowState, so the half a planning sigil
// leaves implicit is stated rather than inferred from the sigil's own spelling.
func composerStatusState(destination app.CaptureDestination) models.WorkflowState {
	if destination == "" {
		destination = app.CaptureDestinationInbox
	}
	return destination.WorkflowState()
}

// View renders whichever presentation the composer currently owns: the expanded
// form over the whole content area, or the bounded capture panel the shell
// stacks under the presentation this capture was raised from.
func (m *TaskComposerModel) View() tea.View {
	if m.Mode == ComposerExpanded && m.Expanded != nil {
		return m.Expanded.View()
	}
	theme.ApplyThemedTextarea(&m.Input)
	return tea.NewView(m.renderCompactPanel(m.compactPanelRows()))
}

func (m *TaskComposerModel) nonblankLineCount() int {
	count := 0
	for _, line := range strings.Split(m.Input.Value(), "\n") {
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
func (m *TaskComposerModel) GetConfirmDialog() *chrome.ConfirmDialog {
	if m.Mode == ComposerExpanded && m.Expanded != nil {
		return m.Expanded.GetConfirmDialog()
	}
	return m.DiscardConfirmState.GetConfirmDialog()
}

// PublishesOwnGuidance answers for the composer's two presentations. The
// expanded form draws its own footer over the whole content area, so the shell
// footer underneath it would be a second instruction set; the compact panel
// publishes through ContextualGuidance and the shell speaks for it.
func (m *TaskComposerModel) PublishesOwnGuidance() bool {
	return m.Mode == ComposerExpanded
}

func (m *TaskComposerModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	if m.Mode == ComposerExpanded {
		return chrome.GuidanceSpec{}, false
	}
	spec := chrome.CaptureGuidanceSpec(
		m.ConfirmDiscard,
		m.IsAutocompleteActive(),
		m.HelpExpanded,
		m.HelpExpanded && m.SyntaxOverflows(),
		m.SyntaxScrollPercent(),
		m.HasDraft(),
	)
	if len(m.Errors) > 0 && !m.ConfirmDiscard && !m.IsAutocompleteActive() && !m.HelpExpanded {
		// A refused capture is the state most likely to hold text the typist
		// wants back, so the exit says what it will really raise rather than
		// implying the line is already gone.
		spec = chrome.RequiredExitGuidanceSpec(
			"Enter retry", chrome.DraftEscapeHint(m.HasDraft()),
			"Ctrl+E retry & edit", "Ctrl+V paste", "? syntax",
		)
	}
	if !m.ConfirmDiscard && !m.IsAutocompleteActive() && !m.HelpExpanded {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(100,
			"Shift+Enter new line", "Ctrl+F expand",
		)...)
		if m.Preset.AllowProjectSwitch && len(m.Preset.AllProjects) > 1 {
			spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(102, "Ctrl+P project")...)
		}
	}
	// No supplemental naming the panel: the divider above already opens with
	// that label and the route row under it states the destination, so a footer
	// repeating either spends the row capture's own keys are read from.
	return spec, true
}

func (m *TaskComposerModel) HandledKeys() []string {
	if m.Mode == ComposerExpanded && m.Expanded != nil {
		return m.Expanded.HandledKeys()
	}
	return []string{"enter", "ctrl+s", "shift+enter", "ctrl+f", "ctrl+p", "ctrl+e", "?", "esc"}
}
