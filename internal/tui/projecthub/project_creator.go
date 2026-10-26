package projecthub

import (
	"fmt"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// ProjectCreatorModel handles creating a new project via TUI.
type ProjectCreatorModel struct {
	pulse.WorkspacePulseState
	runtime *config.RuntimeState
	manager *config.Manager
	service *app.Service

	nameInput textinput.Model
	width     int

	// submitting is set while a creation command is running and cleared when
	// it returns, success or refusal alike. Enter is ignored meanwhile, so a
	// double press cannot race a second creation against the first. The command
	// clears it itself, because the result is delivered to the shell rather
	// than to this model.
	submitting atomic.Bool

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
			if m.hasDraft() {
				m.ConfirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return status.BackMsg{}
			}

		case "enter":
			if !m.submitting.CompareAndSwap(false, true) {
				return m, nil
			}
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
	submitting := &m.submitting
	return func() tea.Msg {
		defer submitting.Store(false)
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

		if manager == nil || service == nil {
			return status.ErrorMsg{Err: fmt.Errorf("project transaction service is unavailable")}
		}
		created, err := engine.CreateProject(runtime, service, uuid.New().String(), name)
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to create project: %w", err)}
		}
		project, transactionID := created.Project, created.TransactionID

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

	var out strings.Builder
	out.WriteString(chrome.RenderFullscreenTitleWithPulse("Create Project", "", m.CurrentWorkspacePulse(), m.width))
	out.WriteString("\n\n")
	out.WriteString(b.String())

	return tea.NewView(out.String())
}

// hasDraft is the one predicate the Esc handler and the footer share, so the
// hint names the act the key will reach.
func (m *ProjectCreatorModel) hasDraft() bool {
	return strings.TrimSpace(m.nameInput.Value()) != ""
}

func (m *ProjectCreatorModel) guidanceSpec() chrome.GuidanceSpec {
	if m.ConfirmDiscard {
		return chrome.DiscardGuidanceSpec()
	}
	// No supplemental: the title row already says Create Project.
	return chrome.RequiredExitGuidanceSpec("Enter create", chrome.DraftEscapeHint(m.hasDraft()))
}

// ContextualGuidance answers route.ContextualGuidanceProvider: the creator
// hands its keys to the shell footer and draws none of its own.
func (m *ProjectCreatorModel) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	return m.guidanceSpec(), true
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

// QuitBlockReason names content a desktop quit would abandon.
func (m *ProjectCreatorModel) QuitBlockReason() string {
	if m.hasDraft() {
		return "Your new project has not been saved."
	}
	return ""
}

func (m *ProjectCreatorModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.ConfirmDiscard {
		return nil
	}
	return chrome.DesktopInput(&m.nameInput, nil)
}
