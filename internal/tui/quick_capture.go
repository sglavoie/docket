package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
// The expanded-syntax state (helpExpanded and the scroll viewport) lives in the
// embedded quickCaptureInput so it is shared with the Linked Task Creator.
type QuickCaptureModel struct {
	quickCaptureInput
	capture CaptureCoordinator
	width   int
	height  int
	discardConfirmState
}

// SetCaptureCoordinator injects the shared capture coordinator (captureWireable).
func (m *QuickCaptureModel) SetCaptureCoordinator(c CaptureCoordinator) { m.capture = c }

// NewQuickCaptureModel creates a new quick capture model.
// initialText pre-populates the input field (e.g., "#tag /section" from active filters).
func NewQuickCaptureModel(project *models.Project, allProjects []*models.Project, fromGlobalCapture bool, initialText ...string) *QuickCaptureModel {
	placeholder := "Type a task title…"
	m := &QuickCaptureModel{
		quickCaptureInput: newQuickCaptureInput(
			project,
			allProjects,
			placeholder,
			fromGlobalCapture,
		),
		width: 80,
	}
	if len(initialText) > 0 && initialText[0] != "" {
		m.input.SetValue(initialText[0])
		m.input.CursorEnd()
		m.updatePreview()
	}
	return m
}

// SetSize sets the view dimensions.
func (m *QuickCaptureModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.quickCaptureInput.width = width
	m.resizeTextarea(width)
	m.setSyntaxViewportWidth(width)
}

func (m *QuickCaptureModel) Init() tea.Cmd {
	return nil
}

func (m *QuickCaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return CloseQuickCaptureMsg{} }
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

		// Interaction precedence: autocomplete owns navigation (Ctrl+J/K, Tab,
		// Enter, Esc) while active; then the expanded syntax guide claims scroll
		// keys; then the ? toggle; then normal capture handling.

		// Handle autocomplete navigation when active
		if m.handleAutocompleteKey(key) {
			return m, nil
		}

		// Scroll the expanded syntax guide while it is actually visible (i.e. not
		// shadowed by autocomplete, which is dismissed above for its nav keys but
		// still owns pgup/pgdown-adjacent handling here).
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

		// Regular key handling
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
			return m, func() tea.Msg { return CloseQuickCaptureMsg{} }

		case "enter":
			// Only create task if not in autocomplete mode
			if m.ac.Mode == acModeNone {
				return m, m.createTask(false, false)
			}

		case "ctrl+d":
			// Create task and open description editor
			if m.ac.Mode == acModeNone {
				return m, m.createTask(true, false)
			}

		case "ctrl+o":
			// Create task and open task detail view
			if m.ac.Mode == acModeNone {
				return m, m.createTask(false, true)
			}

		}
	}

	// Update textarea and refresh autocomplete/preview
	cmd := m.updateInput(msg)
	return m, cmd
}

func (m *QuickCaptureModel) View() tea.View {
	applyThemedTextarea(&m.input)

	autocompleteActive := m.isAutocompleteActive()
	helpVisible := m.helpExpanded && !autocompleteActive

	// Content budget: views are sized to terminal height minus the standard
	// two-row footer, so only reserve the extra rows a wrapped contextual footer
	// adds beyond those two. AppModel.View() owns the single app-shell footer.
	extraFooter := captureFooterExtraRows(m.width, m.confirmDiscard, autocompleteActive, m.helpExpanded)
	budget := max(m.height-extraFooter, 1)

	// Fixed blocks: title, textarea. Both stay put; only the lower pane flexes.
	title := renderFullscreenTitle("Quick Capture", "")
	textareaView := m.input.View()
	// Rows above the flexible region: title + blank + textarea + blank.
	fixedRows := lipgloss.Height(title) + 1 + lipgloss.Height(textareaView) + 1

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

	var b strings.Builder
	b.WriteString(title)
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

// HasTextInput returns true because quick capture uses a textarea input.
func (m *QuickCaptureModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true so the view always handles Esc internally.
func (m *QuickCaptureModel) ShouldCaptureEsc() bool {
	return true
}

// GetConfirmDialog returns the confirm dialog for overlay rendering.
func (m *QuickCaptureModel) IsFullHeight() bool { return false }

func (m *QuickCaptureModel) createTask(openContent, openDetail bool) tea.Cmd {
	// Parsing runs synchronously so a rejected capture keeps the input; the
	// creation itself commits through the application layer (CaptureCoordinator),
	// which loads its own snapshot and reloads the affected project on completion.

	input := m.inputValue()
	if input == "" {
		m.err = fmt.Errorf("task title cannot be empty")
		return nil
	}

	result, err := parseQuickCapture(input, m.project, m.allProjects, m.skipProjectSyntax)
	if err != nil {
		m.err = err
		return nil
	}
	if valErr := models.ValidateTaskTitle(result.title); valErr != nil {
		m.err = valErr
		return nil
	}

	// Determine target project (may be different for cross-project capture).
	targetProjectID := m.project.ID
	if result.targetProjectID != nil {
		targetProjectID = *result.targetProjectID
	}

	// Determine target section. Quick capture only routes to built-in GTD
	// sections; custom grouping is represented with tags. Empty defaults to Inbox.
	targetSection := result.section
	if targetSection == "" {
		targetSection = models.SectionInbox
	} else if !models.IsGTDSection(targetSection) {
		m.err = fmt.Errorf("section not found: %q", targetSection)
		return nil
	}

	req := app.CaptureRequest{
		ProjectID:  targetProjectID,
		Title:      result.title,
		Type:       result.taskType,
		Priority:   result.priority,
		Tags:       result.tags,
		Due:        result.dueDate,
		ParentID:   result.parentID,
		Recurrence: result.recurrence,
		State:      creationStateFromLegacyInputs(targetSection, result.status, ""),
		Prepend:    true,
	}
	return m.capture.CaptureCreate(req, openContent, openDetail)
}

// captureStateForStatus maps a parsed capture status sigil to the workflow-state
// transition the capture use case applies. StatusTodo (and the empty default)
// map to no transition: a freshly created task is already todo, and applying the
// todo transition would Reopen it into the Inbox, clobbering an explicitly
// captured section. The legacy per-status Task.Start/Block/… calls are realized
// by app.applyState via this state. The $start "activate" flag (the retired
// legacy Active field) is intentionally dropped rather than reintroduced.
func captureStateForStatus(status models.TaskStatus) models.WorkflowState {
	if status == "" || status == models.StatusTodo {
		return ""
	}
	return models.TargetStateForStatus(status)
}

// creationStateFromLegacyInputs is the temporary presentation-boundary adapter
// for capture surfaces that still parse section and status vocabulary. Status
// sigils retain precedence, then an explicit route preset, then the section
// bucket. The application service receives exactly one canonical state.
func creationStateFromLegacyInputs(section string, status models.TaskStatus, preset models.WorkflowState) models.WorkflowState {
	if state := captureStateForStatus(status); state != "" {
		return state
	}
	if preset != "" {
		return preset
	}
	if state, ok := stateForSectionName(section); ok {
		return state
	}
	return models.StateInbox
}

func (m *QuickCaptureModel) StatusBarLabel() string { return "Quick Capture" }

// ContextualFooter supplies the app-shell footer for Quick Capture. The view
// renders no local footer; AppModel.View() composes this spec so there is a
// single app-shell footer across all capture states.
func (m *QuickCaptureModel) ContextualFooter() (StatusBarFooterSpec, bool) {
	return captureFooterSpecFull(
		m.confirmDiscard,
		m.isAutocompleteActive(),
		m.helpExpanded,
		m.helpExpanded && m.syntaxOverflows(),
		m.syntaxScrollPercent(),
	), true
}

// HandledKeys returns all keys QuickCaptureModel's Update method can process.
func (m *QuickCaptureModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
