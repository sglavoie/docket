package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectEditorModel handles editing an existing project via TUI.
type ProjectEditorModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager

	// Original values for change detection
	originalName        string
	originalContext     string
	originalDescription string

	// Form fields
	nameInput        textinput.Model
	descriptionInput textinput.Model

	// Context selection (hybrid: selector + custom input)
	contextInput  textinput.Model
	contexts      []string // Available contexts from existing projects
	contextCursor int      // -1 means custom input mode, 0+ means selecting from list

	// Form state
	focusIndex int // 0=name, 1=context, 2=description
	width      int
	height     int
	err        error
	hasChanges bool

	// Discard confirmation state
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewProjectEditorModel creates a new project editor.
func NewProjectEditorModel(project *models.Project, cfg *config.Config, manager *config.Manager) *ProjectEditorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInputWithValue(project.Name, "")
	nameInput.Focus()
	nameInput.CharLimit = 100

	descriptionInput := factory.NewTextInputWithValue(project.Description, "Project description (optional)")
	descriptionInput.CharLimit = 500

	contextInput := factory.NewTextInputWithValue(project.Context, "e.g., personal, work")
	contextInput.CharLimit = 50

	// Get all existing contexts
	contexts := cfg.GetAllContexts()

	// Find current context in list (or -1 for custom)
	contextCursor := -1
	for i, ctx := range contexts {
		if ctx == project.Context {
			contextCursor = i
			break
		}
	}

	return &ProjectEditorModel{
		project:             project,
		config:              cfg,
		configManager:       manager,
		originalName:        project.Name,
		originalContext:     project.Context,
		originalDescription: project.Description,
		nameInput:           nameInput,
		descriptionInput:    descriptionInput,
		contextInput:        contextInput,
		contexts:            contexts,
		contextCursor:       contextCursor,
		focusIndex:          0,
		width:               80,
		height:              24,
	}
}

// checkChanges detects if any field has been modified.
func (m *ProjectEditorModel) checkChanges() bool {
	if strings.TrimSpace(m.nameInput.Value()) != m.originalName {
		return true
	}
	if strings.TrimSpace(m.contextInput.Value()) != m.originalContext {
		return true
	}
	if strings.TrimSpace(m.descriptionInput.Value()) != m.originalDescription {
		return true
	}
	return false
}

// SetSize sets the view dimensions.
func (m *ProjectEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	formFactory.ResizeTextInput(&m.descriptionInput, width)
	formFactory.ResizeTextInput(&m.contextInput, width)
}

func (m *ProjectEditorModel) SupportsHelpShortcut() bool { return false }

func (m *ProjectEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ProjectEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle discard confirmation
	if m.confirmDiscard {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg {
					return BackMsg{}
				}
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.checkChanges() {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (3 fields: name, context, description)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 3
			} else {
				m.focusIndex = (m.focusIndex + 2) % 3
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "enter":
			// Move to next field or save if on last field
			if m.focusIndex < 2 {
				m.blurCurrentField()
				m.focusIndex++
				m.focusCurrentField()
				return m, nil
			}

		case "ctrl+s":
			// Save changes
			return m, m.saveProject()

		case "left", "h":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == 0 {
					// Stay at first context
				} else {
					// From custom mode, go to last context
					m.contextCursor = len(m.contexts) - 1
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}

		case "right", "l":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
					m.contextCursor++
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
				// Stay at last context when at the end
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		oldValue := m.contextInput.Value()
		m.contextInput, cmd = m.contextInput.Update(msg)
		// If user typed something, switch to custom mode
		if m.contextInput.Value() != oldValue {
			m.contextCursor = -1
			// Check if typed value matches an existing context
			for i, ctx := range m.contexts {
				if ctx == m.contextInput.Value() {
					m.contextCursor = i
					break
				}
			}
		}
	case 2:
		m.descriptionInput, cmd = m.descriptionInput.Update(msg)
	}

	// Check for changes
	m.hasChanges = m.checkChanges()

	return m, cmd
}

func (m *ProjectEditorModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.nameInput.Blur()
	case 1:
		m.contextInput.Blur()
	case 2:
		m.descriptionInput.Blur()
	}
}

func (m *ProjectEditorModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.contextInput.Focus()
	case 2:
		m.descriptionInput.Focus()
	}
}

func (m *ProjectEditorModel) saveProject() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("project name cannot be empty")}
		}

		// Generate new slug if name changed
		newSlug := models.Slugify(name)
		if newSlug == "" {
			return ErrorMsg{Err: fmt.Errorf("project name must contain at least one ASCII letter or digit")}
		}
		if name != m.originalName {
			// Check for duplicate slug (excluding self)
			for _, p := range m.config.Projects {
				if p.ID != m.project.ID && p.Slug == newSlug {
					return ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", newSlug)}
				}
			}
		}

		// Rename directory if slug changed
		oldSlug := models.Slugify(m.originalName)
		if oldSlug != newSlug {
			if err := storage.RenameProjectDir(m.project, oldSlug, newSlug, m.config.DataDir); err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to rename project directory: %w", err)}
			}
		}

		// Get context value
		context := strings.TrimSpace(m.contextInput.Value())
		if context == "" {
			context = m.config.DefaultContext
		}

		// Update project fields
		m.project.Name = name
		m.project.Slug = newSlug
		m.project.Context = context
		m.project.Description = strings.TrimSpace(m.descriptionInput.Value())
		m.project.UpdatedAt = time.Now()

		// Save config
		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		return ProjectUpdatedMsg{Project: m.project}
	}
}

func (m *ProjectEditorModel) View() string {
	var b strings.Builder

	// Name input with slug preview
	b.WriteString(renderFormLabelShared("Name:", 0, m.focusIndex))
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	padding := strings.Repeat(" ", FormLabelWidth)
	if name != "" {
		slug := models.Slugify(name)
		slugChanged := slug != models.Slugify(m.originalName)
		if slugChanged {
			b.WriteString(padding)
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Slug: %s ", slug)))
			b.WriteString(InProgressStyle.Render("(will change)"))
		} else {
			b.WriteString(padding)
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Slug: %s", slug)))
		}
	}
	b.WriteString("\n\n")

	// Context input with available contexts hint
	b.WriteString(renderFormLabelShared("Context:", 1, m.focusIndex))
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	// Show available contexts using shared selector
	if len(m.contexts) > 0 {
		b.WriteString(padding)
		b.WriteString(renderContextSelectorShared(m.contexts, m.contextCursor, m.focusIndex == 1))
	}
	b.WriteString("\n\n")

	// Description input
	b.WriteString(renderFormLabelShared("Description:", 2, m.focusIndex))
	b.WriteString(m.descriptionInput.View())
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Build title
	title := "Edit Project"
	if m.hasChanges {
		title += " *"
	}

	// Footer
	left := HelpStyle.Render("Tab/Shift+Tab switch · ←/→ context · Ctrl+S save · Esc cancel")
	right := MutedStyle.Render(title)

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(title, ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return out.String()
}

func (m *ProjectEditorModel) HasTextInput() bool     { return true }
func (m *ProjectEditorModel) ShouldCaptureEsc() bool { return true }
func (m *ProjectEditorModel) IsFullHeight() bool     { return false }

// GetConfirmDialog returns the confirm dialog for overlay rendering.
func (m *ProjectEditorModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}
func (m *ProjectEditorModel) StatusBarLabel() string { return "Edit Project" }
