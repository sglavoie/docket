package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
// The expanded-syntax state (helpExpanded and the scroll viewport) lives in the
// embedded quickCaptureInput so it is shared with the Linked Task Creator.
type QuickCaptureModel struct {
	quickCaptureInput
	width  int
	height int
	discardConfirmState
}

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
	// All mutations run synchronously in the Update() goroutine to avoid
	// data races with View() reading project.Tasks concurrently.

	input := m.inputValue()
	if input == "" {
		m.err = fmt.Errorf("task title cannot be empty")
		return nil
	}

	result, err := parseQuickCapture(input, m.project, m.allProjects, m.skipProjectSyntax)
	if err == nil {
		if valErr := models.ValidateTaskTitle(result.title); valErr != nil {
			m.err = valErr
			return nil
		}
	}
	if err != nil {
		m.err = err
		return nil
	}

	// Determine target project (may be different for cross-project capture)
	targetProject := m.project
	if result.targetProjectID != nil {
		// Find the target project
		for _, p := range m.allProjects {
			if p.ID == *result.targetProjectID {
				targetProject = p
				// Load the target project to get its tasks
				if err := storage.LoadProject(targetProject); err != nil {
					return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to load target project: %w", err)} }
				}
				break
			}
		}
	}

	// Determine target section. Quick capture only routes to built-in GTD
	// sections; custom grouping is represented with tags.
	targetSection := result.section
	if targetSection == "" {
		targetSection = models.SectionInbox
	} else if !models.IsGTDSection(targetSection) {
		m.err = fmt.Errorf("section not found: %q", targetSection)
		return nil
	}

	// Ensure Inbox exists (gets red color)
	if !targetProject.HasSection(models.SectionInbox) {
		targetProject.EnsureSection(models.SectionInbox)
		targetProject.SetSectionColor(models.SectionInbox, "red")
	}
	// Ensure target section exists (for GTD sections other than Inbox)
	if targetSection != models.SectionInbox && !targetProject.HasSection(targetSection) {
		targetProject.EnsureSection(targetSection)
	}
	// Create task
	task := &models.Task{
		ID:              uuid.New().String(),
		ShortID:         targetProject.GetNextShortID(),
		Type:            result.taskType,
		Title:           result.title,
		Status:          models.StatusTodo,
		Priority:        result.priority,
		ParentID:        result.parentID,
		DueDate:         result.dueDate,
		Recurrence:      result.recurrence,
		DependsOn:       []string{},
		RelatedTo:       []string{},
		ScoreDimensions: map[string]int{},
		Children:        []*models.Task{},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	task.SetSection(targetSection)

	for _, tag := range result.tags {
		task.AddTag(tag)
	}

	// Apply status transition using Task methods (handles auto-triage)
	switch result.status {
	case models.StatusInProgress:
		task.Start()
		if result.activate {
			task.Activate()
		}
	case models.StatusBlocked:
		task.Block()
	case models.StatusDone:
		task.Complete()
	case models.StatusCanceled:
		task.Cancel()
	case models.StatusReview:
		task.Review()
	case models.StatusStandby:
		task.Standby()
	case models.StatusMaybe:
		task.Maybe()
	default:
	}

	// Add to project at the beginning so it's prominently visible
	targetProject.Tasks = append([]*models.Task{task}, targetProject.Tasks...)
	targetProject.BuildTaskTree()
	targetProject.ReorderRootTasksByCompletion()

	// Save with backup and reload
	if err := saveProjectSync(targetProject); err != nil {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)} }
	}

	// Also reload the current project if it's different from the target
	// (so that the view refreshes correctly)
	if result.targetProjectID != nil && m.project.ID != *result.targetProjectID {
		if err := storage.LoadProject(m.project); err != nil {
			logging.Warn("failed to reload project %s: %v", m.project.Name, err)
		}
	}

	return func() tea.Msg {
		return TaskCreatedMsg{Task: task, Project: targetProject, OpenContent: openContent, OpenDetail: openDetail}
	}
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
