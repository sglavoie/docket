package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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
	theme.ApplyThemedTextInput(&input)
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
	case ValueProviderTagName:
		// A tag name applies across a whole selection, so there is no single
		// current value to prefill or show.
		return newOperationTextPromptModel(request, label, "", "", "tag name"), nil
	case ValueProviderChecklistItem:
		// Adding appends a new item, so only an edit has a current value; the
		// request's own operation decides which, not the cursor.
		current := ""
		if request.ID == operationChecklistEditItem {
			current = m.checklistItemTextForContext(request.Context)
		}
		return newOperationTextPromptModel(request, label, current, current, "checklist item"), nil
	case ValueProviderLensName:
		// Saving the current view mints a new record, so only a rename has a
		// current value. The request's own operation decides which, because the
		// cursor names a lens in both cases.
		current := ""
		if request.ID == operationLensRename {
			lens, err := m.resolveLensTarget(identity)
			if err != nil {
				return nil, err
			}
			current = lens.Name
		}
		return newOperationTextPromptModel(request, label, current, current, "lens name"), nil
	default:
		return nil, fmt.Errorf("unknown operation text value provider %q", provider)
	}
}

// pendingTextValueProvider registers a text provider with the executor without
// supplying a value, so the shell opens the prompt for it.
func pendingTextValueProvider(OperationInvocation) OperationValueProviderResult {
	return OperationValueProviderResult{}
}
