package operationhost

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// OperationTextPromptModel is the value step for operations whose value is typed
// rather than chosen from a list. It sits at the same overlay depth as the value
// picker and returns the same selection and cancellation messages.
type OperationTextPromptModel struct {
	request  operation.OperationExecutionRequest
	input    textinput.Model
	title    string
	subtitle string
	notice   string
}

func NewOperationTextPromptModel(
	request operation.OperationExecutionRequest,
	title string,
	subtitle string,
	current string,
	placeholder string,
) *OperationTextPromptModel {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = 200
	input.SetValue(current)
	input.Focus()
	theme.ApplyThemedTextInput(&input)
	return &OperationTextPromptModel{
		request:  request,
		input:    input,
		title:    title,
		subtitle: subtitle,
	}
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
		if value == "" {
			model.notice = "Enter a name to continue."
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
