package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// LinkedTaskCreatorModel handles creation of tasks linked to a source task.
// The expanded-syntax flag (helpExpanded) lives in the embedded quickCaptureInput
// so it is shared with Quick Capture; the viewport-backed scrollable guide is
// adopted here in a later session.
type LinkedTaskCreatorModel struct {
	quickCaptureInput
	capture    CaptureCoordinator
	sourceTask *models.Task
	linkType   LinkType
	width      int
	height     int
	discardConfirmState
}

// SetCaptureCoordinator injects the shared capture coordinator (captureWireable).
func (m *LinkedTaskCreatorModel) SetCaptureCoordinator(c CaptureCoordinator) { m.capture = c }

// NewLinkedTaskCreatorModel creates a new linked task creator.
func NewLinkedTaskCreatorModel(sourceTask *models.Task, project *models.Project, linkType LinkType) *LinkedTaskCreatorModel {
	return &LinkedTaskCreatorModel{
		quickCaptureInput: newQuickCaptureInput(
			project,
			nil, // no cross-project capture for linked tasks
			"Type a task title…",
			true, // skip %project syntax
		),
		sourceTask: sourceTask,
		linkType:   linkType,
		width:      80,
	}
}

// SetSize sets the view dimensions.
func (m *LinkedTaskCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.quickCaptureInput.width = width
	m.resizeTextarea(width)
	m.setSyntaxViewportWidth(width)
}

func (m *LinkedTaskCreatorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *LinkedTaskCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		// Interaction precedence mirrors Quick Capture: autocomplete owns its
		// navigation keys while active; then the expanded syntax guide claims
		// scroll keys; then the ? toggle; then normal capture handling.

		// Handle autocomplete navigation when active
		if m.handleAutocompleteKey(key) {
			return m, nil
		}

		// Scroll the expanded syntax guide while it is actually visible (not
		// shadowed by autocomplete).
		if m.helpExpanded && !m.isAutocompleteActive() {
			switch key {
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

		// Toggle syntax help (opens at the top, closes when open).
		if key == "?" && m.ac.Mode == acModeNone {
			m.toggleHelp()
			return m, nil
		}

		switch key {
		case "esc":
			// Esc dismisses the expanded guide before cancel/discard; autocomplete
			// was already dismissed above.
			if m.helpExpanded {
				m.closeHelp()
				return m, nil
			}
			if strings.TrimSpace(m.inputValue()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			if m.ac.Mode == acModeNone {
				return m, m.createTask(false, false)
			}

		case "ctrl+d":
			if m.ac.Mode == acModeNone {
				return m, m.createTask(true, false)
			}

		case "ctrl+o":
			if m.ac.Mode == acModeNone {
				return m, m.createTask(false, true)
			}
		}
	}

	cmd := m.updateInput(msg)
	return m, cmd
}

func (m *LinkedTaskCreatorModel) View() tea.View {
	applyThemedTextarea(&m.input)

	autocompleteActive := m.isAutocompleteActive()
	helpVisible := m.helpExpanded && !autocompleteActive

	// Content budget: views are sized to terminal height minus the standard
	// two-row footer, so only reserve the extra rows a wrapped contextual footer
	// adds beyond those two. AppModel.View() owns the single app-shell footer.
	extraFooter := captureFooterExtraRows(m.width, m.confirmDiscard, autocompleteActive, m.helpExpanded)
	budget := max(m.height-extraFooter, 1)

	// Fixed blocks: title, the source-task context header, and the textarea. All
	// stay put; only the lower pane (autocomplete or syntax viewport) flexes.
	linkName := m.linkTypeName()
	title := renderFullscreenTitle(fmt.Sprintf("New %s", linkName), "")
	header := SubtitleStyle.Render(fmt.Sprintf("Create %s for #%d", linkName, m.sourceTask.ShortID)) + "\n" +
		MutedStyle.Render(truncateToVisualWidth(m.sourceTask.Title, max(m.width-2, 1)))
	textareaView := m.input.View()
	// Rows above the flexible region: title + blank + header + blank + textarea + blank.
	fixedRows := lipgloss.Height(title) + 1 + lipgloss.Height(header) + 1 + lipgloss.Height(textareaView) + 1

	discard := ""
	discardRows := 0
	if m.confirmDiscard {
		discard = ValidationErrorStyle.Render("Discard input? (y/n)")
		discardRows = 1
	}

	remaining := max(budget-fixedRows-discardRows, 1)

	// A lower pane (autocomplete or syntax viewport) needs a minimum height; when
	// present, the full preview may fall back to the compact two-row form.
	lowerMin := 0
	if autocompleteActive || helpVisible {
		lowerMin = min(3, remaining)
	}

	previewBlock, previewRows, blankAfterPreview := m.composePreviewBlock(remaining, lowerMin)

	// Whatever remains after the (possibly compact) preview goes to the lower pane.
	lowerRows := max(remaining-previewRows-blankAfterPreview, 1)
	lower := ""
	switch {
	case autocompleteActive:
		lower = m.ac.RenderDropdownWithLimit("", lowerRows)
	case helpVisible:
		m.setSyntaxViewportHeight(lowerRows)
		lower = m.syntaxVP.View()
	}

	// The Linked Task Creator is a normal (non-full-height) stack view, so
	// AppModel.View() owns the single app-shell footer via ContextualFooter().
	// Rendering an inline footer here would duplicate the app-shell separator.
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(textareaView)
	if previewBlock != "" {
		b.WriteString("\n\n")
		b.WriteString(previewBlock)
	}
	if lower != "" {
		b.WriteString("\n\n")
		b.WriteString(lower)
	}
	if discard != "" {
		b.WriteString("\n")
		b.WriteString(discard)
	}

	return tea.NewView(b.String())
}

// HasTextInput returns true because this view uses a textarea.
func (m *LinkedTaskCreatorModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true so the view always handles Esc internally.
func (m *LinkedTaskCreatorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *LinkedTaskCreatorModel) IsFullHeight() bool { return false }

func (m *LinkedTaskCreatorModel) linkTypeName() string {
	switch m.linkType {
	case LinkTypeBlocker:
		return "Blocker"
	case LinkTypeDependent:
		return "Dependent"
	case LinkTypeSubtask:
		return "Subtask"
	case LinkTypeFollowUp:
		return "Follow-up"
	default:
		return "Linked Task"
	}
}

func (m *LinkedTaskCreatorModel) createTask(openContent, openDetail bool) tea.Cmd {
	// Parsing runs synchronously so a rejected capture keeps the input; the
	// creation itself commits through the application layer (CaptureCoordinator's
	// linked-task use case), which loads its own snapshot and reloads the affected
	// project on completion.

	input := m.inputValue()
	if input == "" {
		m.err = fmt.Errorf("task title cannot be empty")
		return nil
	}

	// Parse inline metadata from input
	result, err := parseQuickCapture(input, m.project, nil, true)
	if err != nil {
		m.err = err
		return nil
	}

	// Validate title to prevent storage corruption from meta-characters
	if err := models.ValidateTaskTitle(result.title); err != nil {
		m.err = err
		return nil
	}

	// Default section inherits the source's bucket; an explicit parsed section
	// overrides it (both must be built-in GTD sections).
	section := taskSectionBucket(m.sourceTask)
	if result.section != "" {
		section = result.section
	}

	// A parsed >parent applies only for non-subtask link types (a subtask's
	// parent is the source itself).
	var parentID *string
	if result.parentID != nil && m.linkType != LinkTypeSubtask {
		parentID = result.parentID
	}

	req := app.LinkedTaskRequest{
		Task: app.CaptureRequest{
			ProjectID:  m.project.ID,
			Title:      result.title,
			Type:       result.taskType,
			Priority:   result.priority,
			Tags:       result.tags,
			Due:        result.dueDate,
			ParentID:   parentID,
			Recurrence: result.recurrence,
			State:      creationStateFromLegacyInputs(section, result.status, ""),
		},
		SourceTaskID: m.sourceTask.ID,
		Link:         appLinkKind(m.linkType),
	}
	return m.capture.CreateLinked(req, m.linkType, openContent, openDetail)
}

// appLinkKind maps the TUI LinkType to the application-layer LinkKind.
func appLinkKind(lt LinkType) app.LinkKind {
	switch lt {
	case LinkTypeBlocker:
		return app.LinkBlocker
	case LinkTypeDependent:
		return app.LinkDependent
	case LinkTypeSubtask:
		return app.LinkSubtask
	case LinkTypeFollowUp:
		return app.LinkFollowUp
	default:
		return app.LinkFollowUp
	}
}

func (m *LinkedTaskCreatorModel) StatusBarLabel() string { return "New Linked Task" }

// ContextualFooter supplies the app-shell footer for the Linked Task Creator,
// sharing the capture footer spec with Quick Capture. The view renders no local
// footer; AppModel.View() composes this spec, including the expanded-syntax
// scroll hints and percentage once the guide overflows its viewport.
func (m *LinkedTaskCreatorModel) ContextualFooter() (StatusBarFooterSpec, bool) {
	return captureFooterSpecFull(
		m.confirmDiscard,
		m.isAutocompleteActive(),
		m.helpExpanded,
		m.helpExpanded && m.syntaxOverflows(),
		m.syntaxScrollPercent(),
	), true
}

// HandledKeys returns all keys LinkedTaskCreatorModel's Update method can process.
func (m *LinkedTaskCreatorModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
