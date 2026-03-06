package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SaveSearchInputModel handles naming a saved search.
type SaveSearchInputModel struct {
	query          string // The search query to save
	input          textinput.Model
	err            string
	width          int
	height         int
	canceled       bool
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewSaveSearchInputModel creates a new save search input model.
func NewSaveSearchInputModel(query string) *SaveSearchInputModel {
	input := textinput.New()
	input.Placeholder = "Enter a name for this search"
	input.Focus()
	input.CharLimit = 50
	input.Width = 40

	return &SaveSearchInputModel{
		query: query,
		input: input,
	}
}

// SetSize sets the view dimensions.
func (m *SaveSearchInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = min(width-20, 40)
}

func (m *SaveSearchInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SaveSearchInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.input.Value()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			m.canceled = true
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.err = "Name cannot be empty"
				return m, nil
			}

			// Send the save message
			return m, func() tea.Msg {
				return SaveSearchMsg{Name: name, Query: m.query}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = "" // Clear error on any input
	return m, cmd
}

func (m *SaveSearchInputModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle("Save Search", ""))
	b.WriteString("\n\n")

	// Show the query being saved
	b.WriteString(MutedStyle.Render("Query: "))
	b.WriteString(BoldStyle.Render(m.query))
	b.WriteString("\n\n")

	// Input field
	b.WriteString(SubtitleStyle.Render("Name"))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Error message if any
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render(m.err))
		b.WriteString("\n")
	}

	// Help footer
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter to save · Esc to cancel"), "", m.width))

	return b.String()
}

// HasTextInput returns true because the save search input is active.
func (m *SaveSearchInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *SaveSearchInputModel) ShouldCaptureEsc() bool { return true }

func (m *SaveSearchInputModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *SaveSearchInputModel) IsFullHeight() bool { return false }
