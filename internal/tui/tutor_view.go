package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TutorModel implements the tutor view stub.
type TutorModel struct {
	width  int
	height int
}

// NewTutorModel creates a new tutor view.
func NewTutorModel() *TutorModel {
	return &TutorModel{}
}

func (m *TutorModel) Init() tea.Cmd {
	return nil
}

func (m *TutorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return BackMsg{} }
		}
	}
	return m, nil
}

func (m *TutorModel) View() string {
	maxWidth := 72
	if m.width < maxWidth {
		maxWidth = m.width
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	title := titleStyle.Render("tt Tutor")
	body := "Welcome to tt! This tutor is under construction. Press q to go back."

	// Wrap body to max width
	content := title + "\n\n" + body

	// Center horizontally
	padLeft := (m.width - maxWidth) / 2
	if padLeft < 0 {
		padLeft = 0
	}

	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		b.WriteString(strings.Repeat(" ", padLeft))
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad vertically to fill height
	rendered := b.String()
	lines := strings.Count(rendered, "\n")
	padTop := (m.height - lines) / 3
	if padTop < 0 {
		padTop = 0
	}

	return strings.Repeat("\n", padTop) + rendered
}

func (m *TutorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
