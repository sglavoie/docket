package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TextEditorModel handles editing a single text field in a popup overlay.
// Used for task title and description editing in task creator/editor forms.
type TextEditorModel struct {
	textarea     textarea.Model
	fieldName    string // "title" or "description"
	initialValue string // for cancel restoration
	width        int
	height       int

	discardConfirmState
}

// NewTextEditorModel creates a new text editor popup.
func NewTextEditorModel(fieldName, currentValue string) *TextEditorModel {
	ta := textarea.New()
	ta.SetValue(currentValue)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	ta.Focus()
	applyThemedTextarea(&ta)

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
	// Expand textarea to fill available vertical space.
	// Reserve lines for: title (2), spacing (1), footer (1) = 4 lines overhead.
	if taHeight := height - 4; taHeight > 5 {
		m.textarea.SetHeight(taHeight)
	}
}

func (m *TextEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *TextEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation
	if m.confirmDiscard {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg {
					return TextEditorDoneMsg{
						FieldName: m.fieldName,
						Value:     m.initialValue,
						Canceled:  true,
					}
				}
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case TextEditorExternalDoneMsg:
		// External editor finished - update textarea with result
		if !msg.Canceled {
			m.textarea.SetValue(msg.Value)
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			// Show confirmation if content changed from initial value
			if m.textarea.Value() != m.initialValue {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return TextEditorDoneMsg{
					FieldName: m.fieldName,
					Value:     m.initialValue,
					Canceled:  true,
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
					Canceled:  false,
				}
			}

		case "ctrl+e":
			// Open external editor
			return m, func() tea.Msg {
				return TextEditorExternalMsg{
					FieldName:    m.fieldName,
					CurrentValue: m.textarea.Value(),
				}
			}

		case "shift+enter":
			m.textarea.InsertString("\n")
			return m, nil
		}
	}

	// Delegate all other keys to textarea (including 'q' and 'enter')
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *TextEditorModel) View() tea.View {
	var b strings.Builder

	// Determine title
	title := "Edit Title"
	if m.fieldName == "description" {
		title = "Edit Description"
	}

	b.WriteString(renderFullscreenTitle(title, ""))
	b.WriteString("\n\n")
	applyThemedTextarea(&m.textarea)
	b.WriteString(m.textarea.View())
	b.WriteString("\n")

	left := HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel")
	b.WriteString(renderStatusBarFooter(left, "", m.width))

	return tea.NewView(b.String())
}

// HasTextInput returns true because the text editor uses a textarea input.
func (m *TextEditorModel) HasTextInput() bool {
	return true
}

func (m *TextEditorModel) ShouldCaptureEsc() bool { return true }

func (m *TextEditorModel) IsFullHeight() bool { return false }

// ShowTextEditorMsg triggers showing the text editor popup.
type ShowTextEditorMsg struct {
	FieldName    string // "title" or "description"
	CurrentValue string
}

// TextEditorDoneMsg signals the text editor has finished.
type TextEditorDoneMsg struct {
	FieldName string
	Value     string
	Canceled  bool
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
	Canceled  bool
}

// ShowDescriptionEditorMsg triggers showing the text editor popup for task description.
type ShowDescriptionEditorMsg struct {
	Task    *models.Task
	Project *models.Project
}

// DescriptionEditorDoneMsg signals the description editor has finished.
type DescriptionEditorDoneMsg struct {
	Task           *models.Task
	Project        *models.Project
	NewDescription string
	Canceled       bool
}

func (m *TextEditorModel) StatusBarLabel() string { return "Edit Text" }
