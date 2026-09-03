package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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
	destination = composerDeferredDestination(parsed, destination)

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
	theme.ApplyThemedTextarea(&m.input)
	contentHeight := m.compactContentHeight()
	project, destination := m.effectiveRouteHeader()
	var b strings.Builder
	b.WriteString(renderFullscreenTitleWithPulse("Task Composer", m.preset.label(), m.currentWorkspacePulse(), m.width))
	b.WriteString("\n\n")
	b.WriteString(theme.SubtitleStyle.Render("Project: ") + textfit.TruncateToVisualWidth(project, max(m.width-12, 1)))
	b.WriteString("\n")
	b.WriteString(theme.SubtitleStyle.Render("Destination: ") + textfit.TruncateToVisualWidth(destination, max(m.width-16, 1)))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(renderHorizontalDivider(m.width))

	var errorsBlock strings.Builder
	for _, lineErr := range m.errors {
		errorsBlock.WriteString(theme.ValidationErrorStyle.Render(fmt.Sprintf("Line %d: %s", lineErr.line, lineErr.message)))
		errorsBlock.WriteString("\n")
	}
	errors := strings.TrimRight(errorsBlock.String(), "\n")
	confirm := ""
	if m.confirmDiscard {
		confirm = theme.ValidationErrorStyle.Render("Discard input? (y/n)")
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
		preview = theme.MutedStyle.Render(fmt.Sprintf("%d tasks · shared attributes apply to every line", count))
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
