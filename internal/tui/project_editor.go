package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

func (m *ProjectEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ProjectEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
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
				} else if m.contextCursor == len(m.contexts)-1 {
					// Stay at last context
				}
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
		if name != m.originalName {
			// Check for duplicate slug (excluding self)
			for _, p := range m.config.Projects {
				if p.ID != m.project.ID && p.Slug == newSlug {
					return ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", newSlug)}
				}
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

	b.WriteString(TitleStyle.Render(fmt.Sprintf("Edit Project: %s", m.originalName)))
	if m.hasChanges {
		b.WriteString(" ")
		b.WriteString(UnsavedChangesStyle.Render("(unsaved changes)"))
	}
	b.WriteString("\n\n")

	// Name input with slug preview
	nameLabel := "Name: "
	if m.focusIndex == 0 {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	if name != "" {
		slug := models.Slugify(name)
		slugChanged := slug != models.Slugify(m.originalName)
		if slugChanged {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("       Slug: %s ", slug)))
			b.WriteString(InProgressStyle.Render("(will change)"))
		} else {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("       Slug: %s", slug)))
		}
	}
	b.WriteString("\n\n")

	// Context input with available contexts hint
	contextLabel := "Context: "
	if m.focusIndex == 1 {
		contextLabel = FieldLabelFocusedStyle.Render("Context: ")
	} else {
		contextLabel = FieldLabelStyle.Render("Context: ")
	}
	b.WriteString(contextLabel)
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	// Show available contexts
	if len(m.contexts) > 0 {
		b.WriteString("         ")
		b.WriteString(MutedStyle.Render("Available: "))
		for i, ctx := range m.contexts {
			if i == m.contextCursor {
				b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("[%s]", ctx)))
			} else {
				b.WriteString(MutedStyle.Render(ctx))
			}
			if i < len(m.contexts)-1 {
				b.WriteString(MutedStyle.Render(", "))
			}
		}
		if m.focusIndex == 1 {
			b.WriteString(HelpStyle.Render("  (←/→ to select, or type custom)"))
		}
	}
	b.WriteString("\n\n")

	// Description input
	descLabel := "Description: "
	if m.focusIndex == 2 {
		descLabel = FieldLabelFocusedStyle.Render("Description: ")
	} else {
		descLabel = FieldLabelStyle.Render("Description: ")
	}
	b.WriteString(descLabel)
	b.WriteString(m.descriptionInput.View())
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Help
	help := BoxStyle.Render(
		BoldStyle.Render("Controls") + "\n" +
			"  Tab/Shift+Tab  Switch field\n" +
			"  Left/Right     Select existing context\n" +
			"  Ctrl+S         Save changes\n" +
			"  Esc            Cancel",
	)
	b.WriteString(help)

	return b.String()
}
