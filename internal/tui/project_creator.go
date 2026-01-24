package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectCreatorModel handles creating a new project via TUI.
type ProjectCreatorModel struct {
	config        *config.Config
	configManager *config.Manager

	nameInput     textinput.Model
	contextInput  textinput.Model
	contextCursor int      // -1 means custom input mode, 0+ means selecting from list
	contexts      []string // Available contexts from existing projects

	focusIndex int // 0 = name input, 1 = context selection
	width      int
	height     int

	canceled bool
}

// NewProjectCreatorModel creates a new project creator.
func NewProjectCreatorModel(cfg *config.Config, manager *config.Manager) *ProjectCreatorModel {
	factory := DefaultFormFieldFactory()
	nameInput := factory.NewTextInputFocused("Project name")
	nameInput.CharLimit = 100

	// Context input with default value
	contextInput := factory.NewTextInputWithValue(cfg.DefaultContext, "e.g., personal, work")
	contextInput.CharLimit = 50

	// Build contexts list: start with default context, then add others
	contexts := []string{cfg.DefaultContext}
	for _, ctx := range cfg.GetAllContexts() {
		if ctx != cfg.DefaultContext {
			contexts = append(contexts, ctx)
		}
	}

	return &ProjectCreatorModel{
		config:        cfg,
		configManager: manager,
		nameInput:     nameInput,
		contextInput:  contextInput,
		contextCursor: 0, // Default context is at index 0
		contexts:      contexts,
		focusIndex:    0,
		width:         80,
		height:        24,
	}
}

// SetSize sets the view dimensions.
func (m *ProjectCreatorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
	formFactory.ResizeTextInput(&m.contextInput, width)
}

func (m *ProjectCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ProjectCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.canceled = true
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			m.focusIndex = (m.focusIndex + 1) % 2
			if m.focusIndex == 0 {
				m.nameInput.Focus()
				m.contextInput.Blur()
			} else {
				m.nameInput.Blur()
				m.contextInput.Focus()
			}
			return m, nil

		case "enter":
			if m.focusIndex == 0 {
				// Move to context selection
				m.focusIndex = 1
				m.nameInput.Blur()
				m.contextInput.Focus()
				return m, nil
			}
			// Submit
			return m, m.createProject()

		case "left", "h":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == -1 {
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
	}

	return m, cmd
}

func (m *ProjectCreatorModel) createProject() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("project name cannot be empty")}
		}

		// Check for duplicate slug
		slug := models.Slugify(name)
		for _, p := range m.config.Projects {
			if p.Slug == slug {
				return ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", slug)}
			}
		}

		// Create project
		projectID := uuid.New().String()
		projectPath := m.config.GetProjectPath(slug)
		project := models.NewProject(projectID, name, projectPath)

		// Get context value
		context := strings.TrimSpace(m.contextInput.Value())
		if context == "" {
			context = m.config.DefaultContext
		}
		project.Context = context

		// Ensure data directory exists
		if err := m.config.EnsureDataDir(); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create data directory: %w", err)}
		}

		// Create project directory and initial tasks.md
		if err := storage.CreateProjectDir(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}

		// Add to config and set as current
		m.config.AddProject(project)
		m.config.CurrentProject = project.ID

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		return ProjectCreatedMsg{Project: project}
	}
}

func (m *ProjectCreatorModel) View() string {
	var b strings.Builder

	// Name input
	var nameLabel string
	if m.focusIndex == 0 {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Context input with available contexts hint
	var contextLabel string
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

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	if name != "" {
		slug := models.Slugify(name)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Slug: %s", slug)))
	}

	// Help footer
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Tab: switch field · ←/→: select context (or type custom) · Enter: submit · Esc: cancel"))

	// Wrap in overlay card
	cardWidth := m.width - 4
	if cardWidth > 60 {
		cardWidth = 60
	}
	config := OverlayCardConfig{
		Title:        "Create Project",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}
