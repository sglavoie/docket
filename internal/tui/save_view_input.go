package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SaveViewInputModel handles naming a saved view.
type SaveViewInputModel struct {
	summary        string // Summary of active filters being saved
	input          textinput.Model
	err            string
	width          int
	height         int
	canceled       bool
	confirmDiscard bool
	confirmOverlay ConfirmDialog
}

// NewSaveViewInputModel creates a new save view input model.
func NewSaveViewInputModel(summary string) *SaveViewInputModel {
	input := textinput.New()
	input.Placeholder = "Enter a name for this view"
	input.Focus()
	input.CharLimit = 50
	input.Width = 40

	return &SaveViewInputModel{
		summary: summary,
		input:   input,
	}
}

// SetSize sets the view dimensions.
func (m *SaveViewInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = min(width-20, 40)
}

func (m *SaveViewInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SaveViewInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

			return m, func() tea.Msg {
				return SaveViewMsg{Name: name}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = ""
	return m, cmd
}

func (m *SaveViewInputModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Save View", ""))
	b.WriteString("\n\n")

	// Show summary of what is being saved
	if m.summary != "" {
		b.WriteString(MutedStyle.Render("Saving: "))
		b.WriteString(BoldStyle.Render(m.summary))
		b.WriteString("\n\n")
	}

	b.WriteString(SubtitleStyle.Render("Name"))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render(m.err))
		b.WriteString("\n")
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter to save · Esc to cancel"), "", m.width))

	return b.String()
}

// HasTextInput returns true because the save view input is active.
func (m *SaveViewInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view (cancels via BackMsg).
func (m *SaveViewInputModel) ShouldCaptureEsc() bool { return true }

func (m *SaveViewInputModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *SaveViewInputModel) IsFullHeight() bool { return false }

// describeSavedViewFilters builds a human-readable summary of a saved view's filter state.
func describeSavedViewFilters(filterType string, filterStatus string, filterPriority int, filterTags []string, filterSections []string, filterDueDate int, filterActionable bool, filterAttrs map[string]string, filterScoreMin float64, sortField int, viewMode int) string {
	var parts []string

	switch viewMode {
	case 1:
		parts = append(parts, "Next")
	case 2:
		parts = append(parts, "Inbox")
	case 3:
		parts = append(parts, "Waiting")
	case 4:
		parts = append(parts, "Someday")
	case 5:
		parts = append(parts, "Completed")
	case 6:
		parts = append(parts, "Snoozed")
	case 7:
		parts = append(parts, "All")
	}

	if filterPriority == 1 {
		parts = append(parts, "!Hi")
	}
	if filterType != "" {
		parts = append(parts, "type:"+filterType)
	}
	if filterStatus != "" {
		parts = append(parts, "status:"+filterStatus)
	}
	if len(filterTags) > 0 {
		for _, tag := range filterTags {
			parts = append(parts, "#"+tag)
		}
	}
	if len(filterSections) > 0 {
		for _, sec := range filterSections {
			parts = append(parts, "@"+sec)
		}
	}
	switch filterDueDate {
	case 1:
		parts = append(parts, "due:overdue")
	case 2:
		parts = append(parts, "due:today")
	case 3:
		parts = append(parts, "due:week")
	case 4:
		parts = append(parts, "due:none")
	}
	if filterActionable {
		parts = append(parts, "ready")
	}
	if len(filterAttrs) > 0 {
		for k, v := range filterAttrs {
			parts = append(parts, k+"="+v)
		}
	}
	if filterScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", filterScoreMin))
	}

	sortNames := map[int]string{1: "priority", 2: "due date", 3: "created", 4: "status", 5: "score"}
	if name, ok := sortNames[sortField]; ok {
		parts = append(parts, "sort:"+name)
	}

	if len(parts) == 0 {
		return "Active (default)"
	}
	return strings.Join(parts, " | ")
}
