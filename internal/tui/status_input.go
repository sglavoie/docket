package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// StatusInputModel handles editing a project's status note.
type StatusInputModel struct {
	project *models.Project
	runtime *config.RuntimeState
	iconKey string
	input   textinput.Model
	width   int
	height  int
}

// NewStatusInputModel creates a new status input model pre-filled with the current status.
func NewStatusInputModel(project *models.Project, runtime *config.RuntimeState) *StatusInputModel {
	iconKey := ""
	if project != nil {
		iconKey = project.StatusIcon
	}
	return newStatusInputModelWithIcon(project, runtime, iconKey)
}

// newStatusInputModelWithIcon creates the second step of the project-status
// draft. The selected icon remains local to the editor until Enter commits the
// icon and status text together.
func newStatusInputModelWithIcon(project *models.Project, runtime *config.RuntimeState, iconKey string) *StatusInputModel {
	input := textinput.New()
	input.Placeholder = "Enter project status"
	input.Focus()
	input.CharLimit = 120
	input.SetWidth(60)
	input.SetValue(project.Description)
	applyThemedTextInput(&input)

	return &StatusInputModel{
		project: project,
		runtime: runtime,
		iconKey: iconKey,
		input:   input,
	}
}

// SetSize sets the view dimensions.
func (m *StatusInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	inputWidth := max(min(width-20, 60), 20)
	m.input.SetWidth(inputWidth)
}

func (m *StatusInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *StatusInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			status := strings.TrimSpace(m.input.Value())
			updated := *m.project
			updated.Description = status
			updated.StatusIcon = m.iconKey
			if m.iconKey != "" {
				updated.StatusIconSource = "user"
			} else {
				updated.StatusIconSource = ""
			}
			updated.UpdatedAt = time.Now()

			if err := commitCatalogToRuntime(m.runtime, func(catalog *config.WorkspaceCatalog) {
				catalog.UpsertProject(&updated)
			}); err != nil {
				return m, func() tea.Msg {
					return ErrorMsg{Err: err}
				}
			}
			copyCatalogMetadata(m.project, &updated)

			return m, func() tea.Msg {
				return StatusUpdatedMsg{}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *StatusInputModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Project Status", ""))
	b.WriteString("\n\n")

	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(m.project.Name))
	b.WriteString("\n\n")

	// Show current status icon if set
	if m.iconKey != "" {
		if icon := models.ProjectStatusIconByKey(m.iconKey); icon != nil {
			fmt.Fprintf(&b, "%s %s", icon.DisplayIcon(), MutedStyle.Render(icon.Label))
			b.WriteString("\n\n")
		}
	}

	b.WriteString(SubtitleStyle.Render("Status"))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter to save · Esc to cancel"), "", m.width))

	return tea.NewView(b.String())
}

// HasTextInput returns true because the status input is active.
func (m *StatusInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view.
func (m *StatusInputModel) ShouldCaptureEsc() bool { return true }

func (m *StatusInputModel) IsFullHeight() bool { return false }

func (m *StatusInputModel) StatusBarLabel() string { return "Set Status" }
