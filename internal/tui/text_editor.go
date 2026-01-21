package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// TextEditorModel handles editing a single text field in a popup overlay.
// Used for task title and description editing in task creator/editor forms.
type TextEditorModel struct {
	textarea     textarea.Model
	fieldName    string // "title" or "description"
	initialValue string // for cancel restoration
	width        int
	height       int
}

// NewTextEditorModel creates a new text editor popup.
func NewTextEditorModel(fieldName, currentValue string) *TextEditorModel {
	ta := textarea.New()
	ta.SetValue(currentValue)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(0, func(lineIdx int) string { return "" })
	ta.Focus()

	return &TextEditorModel{
		textarea:     ta,
		fieldName:    fieldName,
		initialValue: currentValue,
		width:        80,
		height:       24,
	}
}

// SetSize sets the view dimensions.
func (m *TextEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Use same resize logic as other forms for consistent width
	formFactory.ResizeTextarea(&m.textarea, width)
}

func (m *TextEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *TextEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TextEditorExternalDoneMsg:
		// External editor finished - update textarea with result
		if !msg.Cancelled {
			m.textarea.SetValue(msg.Value)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel - restore original value
			return m, func() tea.Msg {
				return TextEditorDoneMsg{
					FieldName: m.fieldName,
					Value:     m.initialValue,
					Cancelled: true,
				}
			}

		case "ctrl+s":
			// Save - return current value
			value := m.textarea.Value()
			// Replace newlines with spaces for title field
			if m.fieldName == "title" {
				value = strings.ReplaceAll(value, "\n", " ")
				value = strings.TrimSpace(value)
			}
			return m, func() tea.Msg {
				return TextEditorDoneMsg{
					FieldName: m.fieldName,
					Value:     value,
					Cancelled: false,
				}
			}

		case "ctrl+o":
			// Open external editor
			return m, func() tea.Msg {
				return TextEditorExternalMsg{
					FieldName:    m.fieldName,
					CurrentValue: m.textarea.Value(),
				}
			}
		}
	}

	// Delegate all other keys to textarea (including 'q' and 'enter')
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *TextEditorModel) View() string {
	var b strings.Builder
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render("Ctrl+S: save · Ctrl+O: external editor · Esc: cancel"))

	// Determine card title
	title := "Edit Title"
	if m.fieldName == "description" {
		title = "Edit Description"
	}

	// Same card width calculation as quick_capture
	cardWidth := m.width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}

	cfg := OverlayCardConfig{
		Title:        title,
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}

	return renderOverlayCard(b.String(), m.width, m.height, cfg)
}

// ShowTextEditorMsg triggers showing the text editor popup.
type ShowTextEditorMsg struct {
	FieldName    string // "title" or "description"
	CurrentValue string
}

// TextEditorDoneMsg signals the text editor has finished.
type TextEditorDoneMsg struct {
	FieldName string
	Value     string
	Cancelled bool
}

// TextEditorExternalMsg requests opening an external editor.
type TextEditorExternalMsg struct {
	FieldName    string
	CurrentValue string
}

// TextEditorExternalDoneMsg returns result from external editor.
type TextEditorExternalDoneMsg struct {
	FieldName string
	Value     string
	Cancelled bool
}
