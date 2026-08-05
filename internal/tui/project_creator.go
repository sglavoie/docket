package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectCreatorModel handles creating a new project via TUI.
type ProjectCreatorModel struct {
	runtime *config.RuntimeState
	manager *config.Manager
	service *app.Service

	nameInput textinput.Model
	width     int

	discardConfirmState
}

// NewProjectCreatorModel creates a project creator over the domain-native
// catalog snapshot. A new project starts with no tags.
func NewProjectCreatorModel(runtime *config.RuntimeState, manager *config.Manager, service *app.Service) *ProjectCreatorModel {
	factory := DefaultFormFieldFactory()
	nameInput := factory.NewTextInputFocused("Project name")
	nameInput.CharLimit = 100

	return &ProjectCreatorModel{
		runtime:   runtime,
		manager:   manager,
		service:   service,
		nameInput: nameInput,
		width:     80,
	}
}

// SetSize sets the view dimensions.
func (m *ProjectCreatorModel) SetSize(width, height int) {
	m.width = width
	formFactory.ResizeTextInputWithReserve(&m.nameInput, width, FormLabelWidth)
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
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "enter":
			return m, m.createProject()
		}
	}

	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m *ProjectCreatorModel) createProject() tea.Cmd {
	name := strings.TrimSpace(m.nameInput.Value())
	runtime := m.runtime
	manager := m.manager
	service := m.service
	return func() tea.Msg {
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("project name cannot be empty")}
		}

		// Check for duplicate slug
		slug := models.Slugify(name)
		if slug == "" {
			return ErrorMsg{Err: fmt.Errorf("project name must contain at least one ASCII letter or digit")}
		}
		if runtime.GetProject(slug) != nil {
			return ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", slug)}
		}

		// Create project
		projectID := uuid.New().String()
		projectPath := runtime.ProjectPath(slug)
		project, err := models.NewProject(projectID, name, projectPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}

		if manager == nil || service == nil {
			return ErrorMsg{Err: fmt.Errorf("project transaction service is unavailable")}
		}

		if ensureErr := runtime.EnsureDataDir(); ensureErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create data directory: %w", ensureErr)}
		}

		catalog := cloneWorkspaceCatalog(runtime.Catalog())
		if upsertErr := catalog.UpsertProject(project); upsertErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to update project catalog: %w", upsertErr)}
		}
		catalogArtifacts, err := catalogDomainArtifact(runtime, catalog)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to encode project catalog: %w", err)}
		}
		document, err := storage.CanonicalProjectDocument(project, nil)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to encode project: %w", err)}
		}
		content, err := document.WriteCurrent()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to encode project: %w", err)}
		}
		requireAbsent := ""
		artifacts := append(catalogArtifacts, storage.ArtifactChange{
			Path:             storage.TasksFilePath(project.Path),
			Data:             []byte(content),
			Mode:             storage.FilePermissions,
			ExpectedRevision: &requireAbsent,
		})
		if err := service.CommitProjectMutation(storage.OpsCommitRequest{
			Artifacts:   artifacts,
			ProjectIDs:  []string{project.ID},
			UndoAction:  "Create Project",
			UndoContext: project.Name,
		}); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}
		if err := runtime.RefreshAfterCommit(); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to refresh project catalog: %w", err)}
		}

		return ProjectCreatedMsg{Project: project}
	}
}

func (m *ProjectCreatorModel) View() tea.View {
	var b strings.Builder

	// Name input
	b.WriteString(renderFormLabelShared("Name:", 0, 0))
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	if name != "" {
		slug := models.Slugify(name)
		b.WriteString(renderProjectSlugPreview(slug, m.width, "", ""))
	}

	// Footer
	left := HelpStyle.Render("Enter submit · Esc cancel")
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

func renderProjectSlugPreview(slug string, width int, indent, suffix string) string {
	const prefix = "Slug: "

	budget := width - len(indent) - len(prefix) - len(suffix)
	if budget < 1 {
		budget = 1
	}

	line := indent + MutedStyle.Render(prefix+truncateToVisualWidth(slug, budget))
	if suffix != "" {
		line += InProgressStyle.Render(suffix)
	}
	return line
}
