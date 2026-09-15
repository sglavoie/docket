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

func (m *TaskComposerModel) effectiveRouteHeader() (project, destination string) {
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
	)
	if len(m.Errors) > 0 && !m.ConfirmDiscard && !m.IsAutocompleteActive() && !m.HelpExpanded {
		spec = chrome.RequiredActionGuidanceSpec(
			"Enter retry", "Ctrl+E retry & edit", "Ctrl+V paste", "? syntax",
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
	if spec.Supplemental == "" {
		spec.Supplemental = m.Preset.label()
	}
	return spec, true
}

func (m *TaskComposerModel) HandledKeys() []string {
	if m.Mode == ComposerExpanded && m.Expanded != nil {
		return m.Expanded.HandledKeys()
	}
	return []string{"enter", "ctrl+s", "shift+enter", "ctrl+f", "ctrl+p", "ctrl+e", "?", "esc"}
}
