package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// OperationTextPromptModel is the value step for operations whose value is typed
// rather than chosen from a list. It sits at the same overlay depth as the value
// picker and returns the same selection and cancellation messages.
type OperationTextPromptModel struct {
	request  OperationExecutionRequest
	input    textinput.Model
	title    string
	subtitle string
	notice   string
}

func newOperationTextPromptModel(
	request OperationExecutionRequest,
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
	applyThemedTextInput(&input)
	return &OperationTextPromptModel{
		request:  request,
		input:    input,
		title:    title,
		subtitle: subtitle,
	}
}

func (model *OperationTextPromptModel) initCmd() tea.Cmd {
	return textinput.Blink
}

func (model *OperationTextPromptModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil {
		return nil, true, OperationValueCanceledMsg{}
	}
	switch msg.String() {
	case "esc":
		return nil, true, OperationValueCanceledMsg{}
	case "enter":
		value := strings.TrimSpace(model.input.Value())
		if value == "" {
			model.notice = "Enter a name to continue."
			return nil, false, nil
		}
		return nil, true, OperationValueSelectedMsg{
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

func (model *OperationTextPromptModel) render(width int) string {
	if model == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 46), max(width-6, 24))
	model.input.SetWidth(max(innerWidth-4, 10))

	var content strings.Builder
	if model.subtitle != "" {
		content.WriteString(MutedStyle.Render(
			"Current: " + truncateToVisualWidth(model.subtitle, max(innerWidth-11, 8)),
		))
		content.WriteString("\n\n")
	}
	content.WriteString(model.input.View())
	if model.notice != "" {
		content.WriteString("\n\n")
		content.WriteString(ValidationErrorStyle.Render(model.notice))
	}

	return renderOverlayBox(
		content.String(),
		innerWidth,
		lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(model.title),
		HelpStyle.Render("Enter apply · Esc back"),
		lipgloss.NewStyle().Foreground(ColorPrimary),
	)
}

// newOperationTextPrompt builds the prompt from the frozen target identity, so a
// target that already vanished fails here instead of after the user types.
func (m *AppModel) newOperationTextPrompt(
	request OperationExecutionRequest,
	provider ValueProviderID,
) (*OperationTextPromptModel, error) {
	identity := currentEntityIdentity(request.Context)
	label := string(provider)
	if descriptor, ok := m.GetOperationRegistry().Descriptor(request.ID); ok {
		label = descriptor.Label
	}
	switch provider {
	case ValueProviderNoteTitle:
		note, err := m.resolveNoteTarget(identity)
		if err != nil {
			return nil, err
		}
		return newOperationTextPromptModel(
			request, label, note.Note.Title, note.Note.Title, "new note title",
		), nil
	default:
		return nil, fmt.Errorf("unknown operation text value provider %q", provider)
	}
}

// pendingTextValueProvider registers a text provider with the executor without
// supplying a value, so the shell opens the prompt for it.
func pendingTextValueProvider(OperationInvocation) OperationValueProviderResult {
	return OperationValueProviderResult{}
}
