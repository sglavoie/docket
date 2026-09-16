package chrome

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TextEditorModel handles editing a single text field in a popup overlay.
// Used for task title and description editing in task creator/editor forms.
type TextEditorModel struct {
	pulse.WorkspacePulseState
	textarea     textarea.Model
	fieldName    string // "title" or "description"
	initialValue string // for cancel restoration
	width        int
	ExternalEdit bool

	DiscardConfirmState
}

// NewTextEditorModel creates a new text editor popup.
func NewTextEditorModel(fieldName, currentValue string) *TextEditorModel {
	ta := textarea.New()
	ta.SetValue(currentValue)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	ta.Focus()
	theme.ApplyThemedTextarea(&ta)

	return &TextEditorModel{
		textarea:     ta,
		fieldName:    fieldName,
		initialValue: currentValue,
		width:        80,
	}
}

// SetSize sets the view dimensions.
func (m *TextEditorModel) SetSize(width, height int) {
	m.width = width
	// Use same resize logic as other forms for consistent width
	FormFactory.ResizeTextarea(&m.textarea, width)
	// Expand the textarea into the rows left after the title/spacing and the
	// currently rendered one- or two-row guidance footer.
	footerHeight := lipgloss.Height(RenderGuidance(m.GuidanceSpec(), width))
	m.textarea.SetHeight(max(height-2-footerHeight, 1))
}

func (m *TextEditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *TextEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle discard confirmation
	if m.ConfirmDiscard {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				m.ConfirmDiscard = false
				return m, func() tea.Msg {
					return TextEditorDoneMsg{
						FieldName: m.fieldName,
						Value:     m.initialValue,
						Canceled:  true,
					}
				}
			case "n", "N", "esc":
				m.ConfirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case TextEditorExternalDoneMsg:
		// External editor finished - update textarea with result
		m.ExternalEdit = false
		if !msg.Canceled {
			m.textarea.SetValue(msg.Value)
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.hasDraft() {
				m.ConfirmDiscard = true
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
			m.ExternalEdit = true
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

	b.WriteString(RenderFullscreenTitleWithPulse(title, "", m.CurrentWorkspacePulse(), m.width))
	b.WriteString("\n\n")
	theme.ApplyThemedTextarea(&m.textarea)
	b.WriteString(m.textarea.View())
	b.WriteString("\n")

	b.WriteString(RenderGuidance(m.GuidanceSpec(), m.width))

	return tea.NewView(b.String())
}

// hasDraft reports edited content, which is what Esc raises the discard
// question over. The handler and the footer read this one predicate.
func (m *TextEditorModel) hasDraft() bool {
	return m.textarea.Value() != m.initialValue
}

func (m *TextEditorModel) GuidanceSpec() GuidanceSpec {
	if m.ConfirmDiscard {
		return DiscardGuidanceSpec()
	}
	spec := RequiredExitGuidanceSpec(
		"Ctrl+S save", DraftEscapeHint(m.hasDraft()),
		"Ctrl+E external editor", "Shift+Enter new line",
	)
	if m.ExternalEdit {
		spec.Supplemental = "External editor open"
	}
	return spec
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

// ShowTaskTextEditorMsg opens the retained narrow description editor for one
// task. Title editing is owned exclusively by the Outliner row editor.
type ShowTaskTextEditorMsg struct {
	Task      *models.Task
	Project   *models.Project
	Operation app.TaskOperation
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

func (m *TextEditorModel) StatusBarLabel() string { return "Edit Text" }
