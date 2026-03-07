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

// StatusInputModel handles editing a project's status note.
type StatusInputModel struct {
	project       *models.Project
	config        *config.Config
	configManager *config.Manager
	input         textinput.Model
	width         int
	height        int
}

// NewStatusInputModel creates a new status input model pre-filled with the current status.
func NewStatusInputModel(project *models.Project, cfg *config.Config, mgr *config.Manager) *StatusInputModel {
	input := textinput.New()
	input.Placeholder = "Enter project status"
	input.Focus()
	input.CharLimit = 120
	input.Width = 60
	input.SetValue(project.Description)

	return &StatusInputModel{
		project:       project,
		config:        cfg,
		configManager: mgr,
		input:         input,
	}
}

// SetSize sets the view dimensions.
func (m *StatusInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	inputWidth := min(width-20, 60)
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.input.Width = inputWidth
}

func (m *StatusInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *StatusInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			status := strings.TrimSpace(m.input.Value())
			m.project.Description = status
			m.project.UpdatedAt = time.Now()

			if err := m.configManager.Save(m.config); err != nil {
				return m, func() tea.Msg {
					return ErrorMsg{Err: err}
				}
			}

			return m, func() tea.Msg {
				return StatusUpdatedMsg{}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *StatusInputModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Project Status", ""))
	b.WriteString("\n\n")

	b.WriteString(MutedStyle.Render("Project: "))
	b.WriteString(BoldStyle.Render(m.project.Name))
	b.WriteString("\n\n")

	// Show current status icon if set
	if m.project.StatusIcon != "" {
		if icon := models.ProjectStatusIconByKey(m.project.StatusIcon); icon != nil {
			fmt.Fprintf(&b, "%s %s", icon.Emoji, MutedStyle.Render(icon.Label))
			b.WriteString("\n\n")
		}
	}

	b.WriteString(SubtitleStyle.Render("Status"))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter to save · Esc to cancel"), "", m.width))

	return b.String()
}

// HasTextInput returns true because the status input is active.
func (m *StatusInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view.
func (m *StatusInputModel) ShouldCaptureEsc() bool { return true }

func (m *StatusInputModel) IsFullHeight() bool { return false }
