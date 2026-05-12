package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
type QuickCaptureModel struct {
	quickCaptureInput
	width        int
	height       int
	helpExpanded bool
	discardConfirmState
}

// NewQuickCaptureModel creates a new quick capture model.
// initialText pre-populates the input field (e.g., "#tag /section" from active filters).
func NewQuickCaptureModel(project *models.Project, allProjects []*models.Project, fromGlobalCapture bool, initialText ...string) *QuickCaptureModel {
	placeholder := "^type Task title !urgent #tag @due/due:fri ~recur >parent /section $status %project"
	if fromGlobalCapture {
		placeholder = "^type Task title !urgent #tag @due/due:fri ~recur >parent /section $status"
	}
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

		// Handle autocomplete navigation when active
		if m.handleAutocompleteKey(key) {
			return m, nil
		}

		// Toggle syntax help
		if key == "?" && m.ac.Mode == acModeNone {
			m.helpExpanded = !m.helpExpanded
			return m, nil
		}

		// Regular key handling
		switch key {
		case "esc":
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
	var b strings.Builder
	applyThemedTextarea(&m.input)
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Show autocomplete suggestions if active
	if ac := m.renderAutocomplete(); ac != "" {
		b.WriteString(ac)
	}

	b.WriteString("\n")

	// Show live preview or error
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	} else if preview := m.renderPreview(); preview != "" {
		b.WriteString(preview)
		b.WriteString("\n\n")
	}

	// Help text
	if m.helpExpanded {
		b.WriteString(m.renderSyntaxGuide())
	} else {
		helpLine := "^type · !N/word · #tag · @date/due: · ~recur · >parent · /section · $status · %project"
		if m.skipProjectSyntax {
			helpLine = "^type · !N/word · #tag · @date/due: · ~recur · >parent · /section · $status"
		}
		b.WriteString(MutedStyle.Render(helpLine))
	}
	b.WriteString("\n")
	var helpText string
	if m.isAutocompleteActive() {
		helpText = "Tab: accept · ↑↓: navigate · Esc: dismiss"
	} else {
		helpText = "Enter: create · Ctrl+O: create & open · Ctrl+D: create & edit desc · ?: help · Esc: cancel"
	}

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Quick Capture", ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.width))

	return tea.NewView(out.String())
}

// renderOverlayContent returns the core input+preview+help content for use inside
// the floating overlay box. It skips the fullscreen title and status bar footer.
func (m *QuickCaptureModel) renderOverlayContent() string {
	var b strings.Builder
	applyThemedTextarea(&m.input)
	b.WriteString(m.input.View())
	b.WriteString("\n")

	if ac := m.renderAutocomplete(); ac != "" {
		b.WriteString(ac)
	}

	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	} else if preview := m.renderPreview(); preview != "" {
		b.WriteString(preview)
		b.WriteString("\n\n")
	}

	if m.helpExpanded {
		b.WriteString(m.renderSyntaxGuide())
	} else {
		helpLine := "^type · !N/word · #tag · @date/due: · ~recur · >parent · /section · $status · %project"
		if m.skipProjectSyntax {
			helpLine = "^type · !N/word · #tag · @date/due: · ~recur · >parent · /section · $status"
		}
		b.WriteString(MutedStyle.Render(helpLine))
	}

	if m.confirmDiscard {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render("Discard input? (y/n)"))
	}

	return b.String()
}

// overlayHelpText returns the footer help text shown in the overlay.
func (m *QuickCaptureModel) overlayHelpText() string {
	if m.isAutocompleteActive() {
		return "Tab: accept · ↑↓: navigate · Esc: dismiss"
	}
	return "Enter: create · Ctrl+O: open · Ctrl+D: edit desc · ?: help · Esc: cancel"
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

func (m *QuickCaptureModel) renderSyntaxGuide() string {
	prefixStyle := MatchHighlightStyle
	type row struct{ prefix, desc, examples string }
	rows := []row{
		{"^type", "Task type", "^epic  ^story  ^task  ^bug  ^idea  · or: type:bug"},
		{"!N/!word", "Priority (1-5)", "!1  !urgent/!high → !1  !normal → !3  !low → !5  · or: pri:2"},
		{"#tag", "Add tag", "#backend  #urgent  (multiple allowed)  · or: tag:backend"},
		{"@date", "Due date", "@tomorrow  @friday  @+3d · or: due:tomorrow  due:fri  due:3d"},
		{"~recur", "Recurrence", "~daily  ~weekly  ~2w  ~monthly  · or: recur:weekly"},
		{">parent", "Parent task", ">42  >Login  (short ID or title prefix)  · or: parent:42"},
		{"/section", "Section", "/next  /inbox  /waiting  /someday  · or: sec:next"},
		{"$status", "Initial status", "$start  $blocked  $done  $review  · or: status:done"},
	}
	if !m.skipProjectSyntax {
		rows = append(rows, row{"%project", "Target project", "%myproject  (slug or name)  · or: p:myproject"})
	}
	var b strings.Builder
	b.WriteString(MutedStyle.Render("Quick Capture Syntax"))
	b.WriteString("\n\n")
	for _, r := range rows {
		b.WriteString("  ")
		b.WriteString(prefixStyle.Render(fmt.Sprintf("%-10s", r.prefix)))
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%-16s", r.desc)))
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render(r.examples))
		b.WriteString("\n")
	}
	return b.String()
}

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

	// Determine target section and custom section
	targetSection := result.section
	var customSection string
	if targetSection == "" {
		targetSection = models.SectionInbox
	} else if !models.IsGTDSection(targetSection) {
		// Custom section: route to Inbox, store as CustomSection
		customSection = targetSection
		targetSection = models.SectionInbox
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
	// Ensure custom section exists
	if customSection != "" && !targetProject.HasSection(customSection) {
		targetProject.EnsureSection(customSection)
	}

	// Create task
	task := &models.Task{
		ID:              uuid.New().String(),
		ShortID:         targetProject.GetNextShortID(),
		Type:            result.taskType,
		Title:           result.title,
		Status:          models.StatusTodo,
		Priority:        result.priority,
		Section:         targetSection,
		CustomSection:   customSection,
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

// HandledKeys returns all keys QuickCaptureModel's Update method can process.
func (m *QuickCaptureModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
