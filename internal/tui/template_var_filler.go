package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/template"
)

// TemplateVarFillerModel prompts the user to fill in template variable values
// before creating a task from a template.
type TemplateVarFillerModel struct {
	tmpl       *template.TaskTemplate
	vars       []template.TemplateVariable
	inputs     []textinput.Model
	focusIndex int
	width      int
	height     int
}

// NewTemplateVarFillerModel creates a var filler for the given template.
func NewTemplateVarFillerModel(tmpl *template.TaskTemplate) *TemplateVarFillerModel {
	vars := template.ExtractVariables(tmpl)
	inputs := make([]textinput.Model, len(vars))
	for i, v := range vars {
		ti := textinput.New()
		ti.Placeholder = v.Name
		ti.CharLimit = 200
		ti.Width = 40
		if v.Default != "" {
			ti.SetValue(v.Default)
		}
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}

	return &TemplateVarFillerModel{
		tmpl:   tmpl,
		vars:   vars,
		inputs: inputs,
	}
}

func (m *TemplateVarFillerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *TemplateVarFillerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s", "enter":
			// Build values map and apply substitution
			values := make(map[string]string, len(m.vars))
			for i, v := range m.vars {
				values[v.Name] = m.inputs[i].Value()
			}
			resolved := template.ApplyVariables(*m.tmpl, values)
			return m, func() tea.Msg {
				return TemplateVarFillerDoneMsg{Template: &resolved}
			}

		case "esc":
			return m, func() tea.Msg {
				return TemplateVarFillerDoneMsg{Canceled: true}
			}

		case "tab", "down":
			m.inputs[m.focusIndex].Blur()
			m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
			m.inputs[m.focusIndex].Focus()
			return m, nil

		case "shift+tab", "up":
			m.inputs[m.focusIndex].Blur()
			m.focusIndex = (m.focusIndex - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[m.focusIndex].Focus()
			return m, nil
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

func (m *TemplateVarFillerModel) View() string {
	var b strings.Builder

	b.WriteString(BoldStyle.Render("Fill in Template Variables"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(m.tmpl.Name))
	b.WriteString("\n\n")

	for i, v := range m.vars {
		label := v.Name
		if v.Default != "" {
			label += MutedStyle.Render(fmt.Sprintf(" (default: %s)", v.Default))
		}
		b.WriteString(label)
		b.WriteString("\n")
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n\n")
	}

	b.WriteString(HelpStyle.Render("Tab/Shift+Tab: navigate · Enter/Ctrl+S: confirm · Esc: cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Width(m.width - 4)

	return boxStyle.Render(b.String())
}

// SetSize implements the View interface.
func (m *TemplateVarFillerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	inputWidth := width - 10
	if inputWidth < 20 {
		inputWidth = 20
	}
	for i := range m.inputs {
		m.inputs[i].Width = inputWidth
	}
}

// HasTextInput returns true since this view has text inputs.
func (m *TemplateVarFillerModel) HasTextInput() bool {
	return true
}

// ShouldCaptureEsc returns true to handle escape internally.
func (m *TemplateVarFillerModel) ShouldCaptureEsc() bool {
	return true
}

// TemplateVarFillerDoneMsg is sent when the variable filler completes or is canceled.
type TemplateVarFillerDoneMsg struct {
	Canceled bool
	Template *template.TaskTemplate // substituted copy, nil if canceled
}
