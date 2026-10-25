package chrome

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The field names a TextEditorModel is opened for. They route the result back
// to whoever asked; the header reads them through textEditorHeader alone.
const (
	TextEditorFieldTitle           = "title"
	TextEditorFieldDescription     = "description"
	TextEditorFieldFormDescription = "formDesc"
)

// textEditorHeader is the one word the header and the breadcrumb label share:
// Edit Title for the title field and Edit Description for every description,
// the task's own or the composer form's draft.
func textEditorHeader(fieldName string) string {
	if fieldName == TextEditorFieldTitle {
		return "Edit Title"
	}
	return "Edit Description"
}

// TextEditorModel handles editing a single text field as a fullscreen surface.
// It is shell-guided: its keys publish through ContextualGuidance and the shell
// footer draws them, so the view itself draws none.
type TextEditorModel struct {
	pulse.WorkspacePulseState
	textarea     textarea.Model
	fieldName    string
	subtitle     string // the task title the edited text belongs to, if any
	initialValue string // for cancel restoration
	width        int
	height       int
	shellFrame   ShellFrameMetrics
	ExternalEdit bool

	DiscardConfirmState
}

// NewTextEditorModel creates a new text editor. subtitle names the task whose
// text is edited and may be empty.
func NewTextEditorModel(fieldName, subtitle, currentValue string) *TextEditorModel {
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
		subtitle:     subtitle,
		initialValue: currentValue,
		width:        80,
	}
}

// SetSize sets the view dimensions.
func (m *TextEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Use same resize logic as other forms for consistent width
	FormFactory.ResizeTextarea(&m.textarea, width)
	m.fitTextarea()
}

// SetShellFrameMetrics answers route.ShellFrameAware: the rows a wrapped or
// feedback-bearing shell footer costs beyond the two this view was sized
// against come out of the textarea, never out of the title.
func (m *TextEditorModel) SetShellFrameMetrics(metrics ShellFrameMetrics) {
	m.shellFrame = metrics
	m.fitTextarea()
}

// fitTextarea expands the textarea into the rows left after the title and its
// spacing, less whatever the shell footer charges beyond its baseline.
func (m *TextEditorModel) fitTextarea() {
	m.textarea.SetHeight(max(m.height-2-m.shellFrame.ExcessRows(), 1))
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
			if m.fieldName == TextEditorFieldTitle {
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

		case "shift+enter", "alt+enter":
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

	b.WriteString(RenderFullscreenTitleWithPulse(textEditorHeader(m.fieldName), m.subtitle, m.CurrentWorkspacePulse(), m.width))
	b.WriteString("\n\n")
	theme.ApplyThemedTextarea(&m.textarea)
	b.WriteString(m.textarea.View())

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
		"Ctrl+E external editor", "Alt+Enter new line",
	)
	if m.ExternalEdit {
		spec.Supplemental = "External editor open"
	}
	return spec
}

// ContextualGuidance answers route.ContextualGuidanceProvider: the editor hands
// its keys to the shell footer and draws none of its own.
func (m *TextEditorModel) ContextualGuidance() (GuidanceSpec, bool) {
	return m.GuidanceSpec(), true
}

// HasTextInput returns true because the text editor uses a textarea input.
func (m *TextEditorModel) HasTextInput() bool {
	return true
}

func (m *TextEditorModel) ShouldCaptureEsc() bool { return true }

func (m *TextEditorModel) IsFullHeight() bool { return false }

// ShowTextEditorMsg triggers showing the text editor.
type ShowTextEditorMsg struct {
	FieldName    string // one of the TextEditorField constants
	Subtitle     string // the task title the text belongs to, if any
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

func (m *TextEditorModel) StatusBarLabel() string { return textEditorHeader(m.fieldName) }

// QuitBlockReason names content a desktop quit would abandon.
func (m *TextEditorModel) QuitBlockReason() string {
	if m.ExternalEdit {
		return "An external editor is still open."
	}
	if m.hasDraft() {
		return "The " + textEditorHeader(m.fieldName) + " draft has unsaved changes."
	}
	return ""
}

func (m *TextEditorModel) DesktopTextBuffer() *DesktopTextBuffer {
	if m.ConfirmDiscard || m.ExternalEdit {
		return nil
	}
	return DesktopTextarea(&m.textarea, nil)
}
