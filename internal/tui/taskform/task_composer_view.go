package taskform

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/theme"
)

type composerEffectiveRoute struct {
	projectID   string
	projectName string
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

	route := composerEffectiveRoute{destination: destination}
	if project != nil {
		route.projectID = project.ID
		route.projectName = project.Name
	} else {
		route.projectName = "Unavailable"
	}
	return route
}

// composerMixedValue is what one axis reports when the lines being captured do
// not agree about it. Each axis answers for itself, so a batch heading for Today
// and for Done is mixed about its status and mixed about its plan, while one
// heading for Today and for Next agrees about status and not about plan.
const composerMixedValue = "Mixed"

// effectiveRouteHeader states the route as the two independent things it is: the
// planning collection the capture commits to, and the lifecycle state it lands
// in. One destination decides both — `$today` is a plan with a Backlog status —
// so a single value naming only the sigil left the second half unsaid.
//
// An empty plan is the absence of a membership rather than an unknown one, and
// the row drops that half entirely; a status is never absent, since every task
// created is in some state.
func (m *TaskComposerModel) effectiveRouteHeader() (project, plan, status string) {
	var routes []composerEffectiveRoute
	for _, line := range strings.Split(m.Input.Value(), "\n") {
		if strings.TrimSpace(line) != "" {
			routes = append(routes, m.effectiveRouteForLine(line))
		}
	}
	if len(routes) == 0 {
		routes = append(routes, m.effectiveRouteForLine(""))
	}

	project = routes[0].projectName
	plan = composerPlanValue(routes[0].destination)
	status = composerStatusValue(routes[0].destination)
	for _, route := range routes[1:] {
		if route.projectID != routes[0].projectID {
			project = composerMixedValue
		}
		if composerPlanValue(route.destination) != plan && plan != composerMixedValue {
			plan = composerMixedValue
		}
		if composerStatusValue(route.destination) != status && status != composerMixedValue {
			status = composerMixedValue
		}
	}
	return project, plan, status
}

// composerPlanValue names the planning collection the effective route commits
// to and nothing at all for the eight destinations that name a state alone.
func composerPlanValue(destination app.CaptureDestination) string {
	if !destination.IsPlanning() {
		return ""
	}
	return chrome.PlanStatusValue(string(destination))
}

// composerStatusValue names the lifecycle state the effective route lands in.
// Today and Next resolve through WorkflowState, so the half a planning sigil
// leaves implicit is stated rather than inferred from the sigil's own spelling.
func composerStatusValue(destination app.CaptureDestination) string {
	if destination == "" {
		destination = app.CaptureDestinationInbox
	}
	return chrome.PlanStatusValue(destination.WorkflowState().String())
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
