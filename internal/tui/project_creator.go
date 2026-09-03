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
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// ProjectCreatorModel handles creating a new project via TUI.
type ProjectCreatorModel struct {
	pulse.WorkspacePulseState
	runtime *config.RuntimeState
	manager *config.Manager
	service *app.Service

	nameInput textinput.Model
	width     int

	chrome.DiscardConfirmState
}

// NewProjectCreatorModel creates a project creator over the domain-native
// catalog snapshot. A new project starts with no tags.
func NewProjectCreatorModel(runtime *config.RuntimeState, manager *config.Manager, service *app.Service) *ProjectCreatorModel {
	factory := chrome.DefaultFormFieldFactory()
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
	chrome.FormFactory.ResizeTextInputWithReserve(&m.nameInput, width, theme.FormLabelWidth)
}

func (m *ProjectCreatorModel) SupportsHelpShortcut() bool { return false }

func (m *ProjectCreatorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ProjectCreatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle discard confirmation
	if m.ConfirmDiscard {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.ConfirmDiscard = false
				return m, func() tea.Msg {
					return status.BackMsg{}
				}
			case "n", "N", "esc":
				m.ConfirmDiscard = false
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
				m.ConfirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return status.BackMsg{}
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
			return status.ErrorMsg{Err: fmt.Errorf("project name cannot be empty")}
		}

		// Check for duplicate slug
		slug := models.Slugify(name)
		if slug == "" {
			return status.ErrorMsg{Err: fmt.Errorf("project name must contain at least one ASCII letter or digit")}
		}
		if runtime.GetProject(slug) != nil {
			return status.ErrorMsg{Err: fmt.Errorf("project with slug '%s' already exists", slug)}
		}

		// Create project
		projectID := uuid.New().String()
		projectPath := runtime.ProjectPath(slug)
		project, err := models.NewProject(projectID, name, projectPath)
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}

		if manager == nil || service == nil {
			return status.ErrorMsg{Err: fmt.Errorf("project transaction service is unavailable")}
		}

		if ensureErr := runtime.EnsureDataDir(); ensureErr != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to create data directory: %w", ensureErr)}
		}

		catalog := cloneWorkspaceCatalog(runtime.Catalog())
		if upsertErr := catalog.UpsertProject(project); upsertErr != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to update project catalog: %w", upsertErr)}
		}
		catalogArtifacts, err := catalogDomainArtifact(runtime, catalog)
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to encode project catalog: %w", err)}
		}
		document, err := storage.CanonicalProjectDocument(project, nil)
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to encode project: %w", err)}
		}
		content, err := document.WriteCurrent()
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to encode project: %w", err)}
		}
		requireAbsent := ""
		artifacts := append(catalogArtifacts, storage.ArtifactChange{
			Path:             storage.TasksFilePath(project.Path),
			Data:             []byte(content),
			Mode:             storage.FilePermissions,
			ExpectedRevision: &requireAbsent,
		})
		transactionID, err := service.CommitProjectMutationResult(storage.OpsCommitRequest{
			Artifacts:   artifacts,
			ProjectIDs:  []string{project.ID},
			UndoAction:  "Create Project",
			UndoContext: project.Name,
		})
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}
		if err := runtime.RefreshAfterCommit(); err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to refresh project catalog: %w", err)}
		}

		return ProjectCreatedMsg{Project: project, TransactionID: transactionID}
	}
}

func (m *ProjectCreatorModel) View() tea.View {
	var b strings.Builder

	// Name input
	b.WriteString(chrome.RenderFormLabelShared("Name:", 0, 0))
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Show slug preview
	name := strings.TrimSpace(m.nameInput.Value())
	if name != "" {
		slug := models.Slugify(name)
		b.WriteString(renderProjectSlugPreview(slug, m.width, "", ""))
	}

	guidance := chrome.RenderGuidance(m.guidanceSpec(), m.width)

	var out strings.Builder
	out.WriteString(chrome.RenderFullscreenTitleWithPulse("Create Project", "", m.CurrentWorkspacePulse(), m.width))
	out.WriteString("\n\n")
	out.WriteString(b.String())
	out.WriteString("\n")
	out.WriteString(guidance)

	return tea.NewView(out.String())
}

func (m *ProjectCreatorModel) guidanceSpec() chrome.GuidanceSpec {
	if m.ConfirmDiscard {
		return chrome.ConfirmationGuidanceSpec()
	}
	spec := chrome.RequiredActionGuidanceSpec("Enter create")
	spec.Supplemental = "Create Project"
	return spec
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

	line := indent + theme.MutedStyle.Render(prefix+textfit.TruncateToVisualWidth(slug, budget))
	if suffix != "" {
		line += theme.InProgressStyle.Render(suffix)
	}
	return line
}
