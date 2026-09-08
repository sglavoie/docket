package operationhost

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// OperationTextPromptOptions carries the two rules that differ between the
// values this one prompt collects. Both are explicit rather than defaulted:
// every caller states the bound it means, so a new prompt cannot inherit a name
// prompt's limit by saying nothing, and reading a call site tells you whether
// an emptied buffer is a refusal or a clear.
type OperationTextPromptOptions struct {
	// MaxLength bounds the typed value in characters.
	MaxLength int
	// AcceptsEmpty admits the empty value as a real answer. When it is false the
	// prompt holds the buffer open on Enter and shows EmptyNotice instead.
	AcceptsEmpty bool
	// EmptyNotice is what a refused empty buffer says. It is read only when
	// AcceptsEmpty is false.
	EmptyNotice string
}

// NameTextPromptOptions is the rule every prompt that collects a name has always
// obeyed: 200 characters, and a blank buffer is not an answer because there is
// nothing an unnamed note, tag, checklist item, or lens could mean.
func NameTextPromptOptions() OperationTextPromptOptions {
	return OperationTextPromptOptions{
		MaxLength:    200,
		AcceptsEmpty: false,
		EmptyNotice:  "Enter a name to continue.",
	}
}

// ResumeNoteTextPromptOptions is the resume note's own rule. It accepts the
// empty value because absence is the only spelling of "no resume note", so
// emptying the buffer is how a note is cleared, and it carries the model's own
// bound rather than the name prompt's: models.ValidateResumeNote is what the
// stored note must pass, and a prompt that let a longer note be typed would be
// promising something the writer refuses.
func ResumeNoteTextPromptOptions() OperationTextPromptOptions {
	return OperationTextPromptOptions{
		MaxLength:    models.MaxResumeNoteLength,
		AcceptsEmpty: true,
	}
}

// OperationTextPromptModel is the value step for operations whose value is typed
// rather than chosen from a list. It sits at the same overlay depth as the value
// picker and returns the same selection and cancellation messages.
type OperationTextPromptModel struct {
	request  operation.OperationExecutionRequest
	input    textinput.Model
	title    string
	subtitle string
	notice   string
	options  OperationTextPromptOptions
}

func NewOperationTextPromptModel(
	request operation.OperationExecutionRequest,
	title string,
	subtitle string,
	current string,
	placeholder string,
	options OperationTextPromptOptions,
) *OperationTextPromptModel {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = options.MaxLength
	input.SetValue(current)
	input.Focus()
	theme.ApplyThemedTextInput(&input)
	return &OperationTextPromptModel{
		request:  request,
		input:    input,
		title:    title,
		subtitle: subtitle,
		options:  options,
	}
}

// Restore reopens a prompt on the text the user already typed and says why the
// save did not land. A commit can fail after the overlay closed — a stale
// revision, a validation the surface could not have run — and retyping a note
// the user already wrote is the wrong thing to ask of them.
func (model *OperationTextPromptModel) Restore(value, notice string) {
	if model == nil {
		return
	}
	model.input.SetValue(value)
	model.notice = notice
}

// Value is the buffer as it currently stands, and Notice is whatever the prompt
// is currently saying about it. Both are reads of state Render already shows;
// they exist so a caller can inspect the overlay it mounted without parsing the
// box back out of the rendered screen.
func (model *OperationTextPromptModel) Value() string {
	if model == nil {
		return ""
	}
	return model.input.Value()
}

func (model *OperationTextPromptModel) Notice() string {
	if model == nil {
		return ""
	}
	return model.notice
}

func (model *OperationTextPromptModel) InitCmd() tea.Cmd {
	return textinput.Blink
}

func (model *OperationTextPromptModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil {
		return nil, true, operation.OperationValueCanceledMsg{}
	}
	switch msg.String() {
	case "esc":
		return nil, true, operation.OperationValueCanceledMsg{}
	case "enter":
		value := strings.TrimSpace(model.input.Value())
		if value == "" && !model.options.AcceptsEmpty {
			model.notice = model.options.EmptyNotice
			return nil, false, nil
		}
		return nil, true, operation.OperationValueSelectedMsg{
			Request: model.request,
			Value:   value,
		}
	default:
		model.notice = ""
		var cmd tea.Cmd
		model.input, cmd = model.input.Update(msg)
		return cmd, false, nil
	}
}

func (model *OperationTextPromptModel) Render(width int) string {
	if model == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 46), max(width-6, 24))
	model.input.SetWidth(max(innerWidth-4, 10))

	var content strings.Builder
	if model.subtitle != "" {
		content.WriteString(theme.MutedStyle.Render(
			"Current: " + textfit.TruncateToVisualWidth(model.subtitle, max(innerWidth-11, 8)),
		))
		content.WriteString("\n\n")
	}
	content.WriteString(model.input.View())
	if model.notice != "" {
		content.WriteString("\n\n")
		content.WriteString(theme.ValidationErrorStyle.Render(model.notice))
	}

	return chrome.RenderOverlayBox(
		content.String(),
		innerWidth,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(model.title),
		chrome.RenderGuidanceText(chrome.GuidanceSpec{
			Hints: chrome.RequiredGuidanceHints("Enter apply", "Esc back"),
			Rows:  2,
		}, max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}
