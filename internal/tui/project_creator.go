package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	contextCursor int
	contexts      []string

	focusIndex int // 0 = name input, 1 = context selection
	width      int
	height     int

	cancelled bool
}

// NewProjectCreatorModel creates a new project creator.
func NewProjectCreatorModel(cfg *config.Config, manager *config.Manager) *ProjectCreatorModel {
	factory := DefaultFormFieldFactory()
	nameInput := factory.NewTextInputFocused("Project name")
	nameInput.CharLimit = 100

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
		contextCursor: 0,
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
			m.cancelled = true
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			m.focusIndex = (m.focusIndex + 1) % 2
			if m.focusIndex == 0 {
				m.nameInput.Focus()
			} else {
				m.nameInput.Blur()
			}
			return m, nil

		case "enter":
			if m.focusIndex == 0 {
				// Move to context selection
				m.focusIndex = 1
				m.nameInput.Blur()
				return m, nil
			}
			// Submit
			return m, m.createProject()

		case "left", "h":
			if m.focusIndex == 1 && m.contextCursor > 0 {
				m.contextCursor--
			}

		case "right", "l":
			if m.focusIndex == 1 && m.contextCursor < len(m.contexts)-1 {
				m.contextCursor++
			}
		}
	}

	if m.focusIndex == 0 {
		m.nameInput, cmd = m.nameInput.Update(msg)
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
		project.Context = m.contexts[m.contextCursor]

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
	nameLabel := "Name: "
	if m.focusIndex == 0 {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Context selection
	contextLabel := "Context: "
	if m.focusIndex == 1 {
		contextLabel = FieldLabelFocusedStyle.Render("Context: ")
	} else {
		contextLabel = FieldLabelStyle.Render("Context: ")
	}
	b.WriteString(contextLabel)

	for i, ctx := range m.contexts {
		var style lipgloss.Style
		if i == m.contextCursor {
			if m.focusIndex == 1 {
				style = SelectorOptionSelectedStyle
			} else {
				style = BoldStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("[%s]", ctx)))
		} else {
			b.WriteString(SelectorOptionStyle.Render(fmt.Sprintf(" %s ", ctx)))
		}
		b.WriteString(" ")
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
	b.WriteString(MutedStyle.Render("Tab: switch field · ←/→: select context · Enter: submit · Esc: cancel"))

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
