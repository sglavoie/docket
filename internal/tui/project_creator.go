package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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

	discardConfirmState
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

func (m *ProjectCreatorModel) SupportsHelpShortcut() bool { return false }

func (m *ProjectCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ProjectCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle discard confirmation
	if m.confirmDiscard {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				m.canceled = true
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
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.nameInput.Value()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
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
	name := strings.TrimSpace(m.nameInput.Value())
	contextValue := strings.TrimSpace(m.contextInput.Value())
	cfg := m.config
	configManager := m.configManager
	return func() tea.Msg {
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("project name cannot be empty")}
		}

		// Check for duplicate slug
		slug := models.Slugify(name)
		if slug == "" {
			return ErrorMsg{Err: fmt.Errorf("project name must contain at least one ASCII letter or digit")}
		}
		for _, p := range cfg.Projects {
			if p.Slug == slug {
				return ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", slug)}
			}
		}

		// Create project
		projectID := uuid.New().String()
		projectPath := cfg.GetProjectPath(slug)
		project, err := models.NewProject(projectID, name, projectPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}

		// Get context value
		if contextValue == "" {
			contextValue = cfg.DefaultContext
		}
		project.Context = contextValue

		// Ensure data directory exists
		if err := cfg.EnsureDataDir(); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create data directory: %w", err)}
		}

		// Create project directory and initial tasks.md
		if err := storage.CreateProjectDir(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}

		// Add to config and set as current
		cfg.AddProject(project)
		cfg.CurrentProject = project.ID

		if err := configManager.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		return ProjectCreatedMsg{Project: project}
	}
}

func (m *ProjectCreatorModel) View() tea.View {
	var b strings.Builder

	// Name input
	b.WriteString(renderFormLabelShared("Name:", 0, m.focusIndex))
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Context input with available contexts hint
	b.WriteString(renderFormLabelShared("Context:", 1, m.focusIndex))
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	// Show available contexts using shared selector
	if len(m.contexts) > 0 {
		padding := strings.Repeat(" ", FormLabelWidth)
		b.WriteString(padding)
		b.WriteString(renderContextSelectorShared(m.contexts, m.contextCursor, m.focusIndex == 1))
	}
	b.WriteString("\n\n")

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	if name != "" {
		slug := models.Slugify(name)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Slug: %s", slug)))
	}

	// Footer
	left := HelpStyle.Render("Tab switch · ←/→ context · Enter submit · Esc cancel")
	right := MutedStyle.Render("Create Project")

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Create Project", ""))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}

func (m *ProjectCreatorModel) HasTextInput() bool     { return true }
func (m *ProjectCreatorModel) ShouldCaptureEsc() bool { return true }
func (m *ProjectCreatorModel) IsFullHeight() bool     { return false }

func (m *ProjectCreatorModel) StatusBarLabel() string { return "New Project" }
