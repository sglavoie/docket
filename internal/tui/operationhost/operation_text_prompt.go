package operationhost

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
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
	// Multiline collects the value in a textarea instead of a single-line input,
	// which moves the submit key: Enter becomes an ordinary newline and Ctrl+S is
	// what applies. A value whose stored form may hold newlines has to be typed
	// the same way, or the prompt would be refusing spellings the writer accepts.
	Multiline bool
	// DiscardPrompt is the question an Esc on a non-empty buffer asks before it
	// throws the text away. An empty DiscardPrompt means Esc cancels outright,
	// which is right for a short value nobody minds retyping.
	DiscardPrompt string
	// SubtitleLabel names what the subtitle line is showing. It defaults to
	// "Current" because most prompts open on the value they are replacing; a
	// prompt whose subtitle names its target instead has to say so.
	SubtitleLabel string
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

// CommentTextPromptOptions is the quick-comment rule. The bound is the Comments
// panel's own, passed in rather than restated here because that panel owns the
// limit and two spellings of it would drift; a comment is multiline because the
// stored text is, it refuses an empty buffer because the application layer does,
// and it confirms a discard because a typed paragraph is worth a question.
func CommentTextPromptOptions(maxLength int) OperationTextPromptOptions {
	return OperationTextPromptOptions{
		MaxLength:     maxLength,
		AcceptsEmpty:  false,
		EmptyNotice:   "Enter comment text to continue.",
		Multiline:     true,
		DiscardPrompt: "Discard comment?",
		SubtitleLabel: "Task",
	}
}

// operationTextPromptMultilineHeight is how many rows the multiline buffer
// shows. It matches the Comments panel's own add-mode textarea so the same text
// wraps the same way whichever route typed it.
const operationTextPromptMultilineHeight = 4

// discardTextPromptAction names the confirmation the prompt arms for itself. The
// dialog needs a non-empty action to count as active and only this prompt reads
// it back, so the string never leaves this file.
const discardTextPromptAction = "discard-prompt"

// OperationTextPromptModel is the value step for operations whose value is typed
// rather than chosen from a list. It sits at the same overlay depth as the value
// picker and returns the same selection and cancellation messages.
type OperationTextPromptModel struct {
	request  operation.OperationExecutionRequest
	input    textinput.Model
	area     textarea.Model
	title    string
	subtitle string
	notice   string
	options  OperationTextPromptOptions
	confirm  chrome.ConfirmDialog
	// submitted is what makes double submission impossible rather than merely
	// unlikely: the prompt emits one selection and then answers every later
	// submit key with nothing, until Restore reopens it for a retry. A shell
	// that unmounts the overlay is doing the same thing, but only after the
	// message it already sent has been handled.
	submitted bool
}

func NewOperationTextPromptModel(
	request operation.OperationExecutionRequest,
	title string,
	subtitle string,
	current string,
	placeholder string,
	options OperationTextPromptOptions,
) *OperationTextPromptModel {
	model := &OperationTextPromptModel{
		request:  request,
		title:    title,
		subtitle: subtitle,
		options:  options,
	}
	if options.Multiline {
		area := textarea.New()
		area.Placeholder = placeholder
		area.CharLimit = options.MaxLength
		area.SetHeight(operationTextPromptMultilineHeight)
		area.ShowLineNumbers = false
		area.SetValue(current)
		area.Focus()
		theme.ApplyThemedTextarea(&area)
		model.area = area
		return model
	}
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = options.MaxLength
	input.SetValue(current)
	input.Focus()
	theme.ApplyThemedTextInput(&input)
	model.input = input
	return model
}

// Restore reopens a prompt on the text the user already typed and says why the
// save did not land. A commit can fail after the overlay closed — a stale
// revision, a validation the surface could not have run — and retyping a note
// the user already wrote is the wrong thing to ask of them.
func (model *OperationTextPromptModel) Restore(value, notice string) {
	if model == nil {
		return
	}
	if model.options.Multiline {
		model.area.SetValue(value)
	} else {
		model.input.SetValue(value)
	}
	model.notice = notice
	model.submitted = false
	model.confirm.Cancel()
}

// setBufferValue and bufferValue are the one place the two widgets differ to a
// reader of the rest of this file.
func (model *OperationTextPromptModel) bufferValue() string {
	if model.options.Multiline {
		return model.area.Value()
	}
	return model.input.Value()
}

// submitKey is which press applies. Multiline owns Enter for newlines, so it
// cannot also mean apply.
func (model *OperationTextPromptModel) submitKey() string {
	if model.options.Multiline {
		return "ctrl+s"
	}
	return "enter"
}

// Value is the buffer as it currently stands, and Notice is whatever the prompt
// is currently saying about it. Both are reads of state Render already shows;
// they exist so a caller can inspect the overlay it mounted without parsing the
// box back out of the rendered screen.
func (model *OperationTextPromptModel) Value() string {
	if model == nil {
		return ""
	}
	return model.bufferValue()
}

func (model *OperationTextPromptModel) Notice() string {
	if model == nil {
		return ""
	}
	return model.notice
}

func (model *OperationTextPromptModel) InitCmd() tea.Cmd {
	if model != nil && model.options.Multiline {
		return textarea.Blink
	}
	return textinput.Blink
}

// ConfirmingDiscard reports whether the prompt is holding an armed discard
// question. It exists so a caller can see the state Render already shows.
func (model *OperationTextPromptModel) ConfirmingDiscard() bool {
	return model != nil && model.confirm.IsActive()
}

func (model *OperationTextPromptModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil {
		return nil, true, operation.OperationValueCanceledMsg{}
	}
	key := msg.String()
	// An armed discard owns every key: y throws the text away, n and Esc give it
	// back, and nothing else reaches the buffer underneath.
	switch chrome.ResolveConfirmKey(&model.confirm, key) {
	case chrome.ConfirmAnswerConfirmed:
		return nil, true, operation.OperationValueCanceledMsg{}
	case chrome.ConfirmAnswerCanceled, chrome.ConfirmAnswerSwallowed:
		return nil, false, nil
	case chrome.ConfirmAnswerUnarmed:
	}
	switch key {
	case "esc":
		if model.options.DiscardPrompt != "" && strings.TrimSpace(model.bufferValue()) != "" {
			model.confirm.Show(discardTextPromptAction, model.options.DiscardPrompt)
			return nil, false, nil
		}
		return nil, true, operation.OperationValueCanceledMsg{}
	case model.submitKey():
		return model.submit()
	default:
		model.notice = ""
		var cmd tea.Cmd
		if model.options.Multiline {
			model.area, cmd = model.area.Update(msg)
		} else {
			model.input, cmd = model.input.Update(msg)
		}
		return cmd, false, nil
	}
}

// HandlePaste puts a terminal bracketed paste into the same buffer HandleKey
// types into. A paste arrives as its own message rather than as a run of key
// presses, so a prompt that only read key presses would drop a pasted paragraph
// into whatever view the overlay is covering. Pasting is not submitting: the
// buffer stays open and the widget's own CharLimit truncates, so pasted text
// obeys exactly the bound typed text does.
func (model *OperationTextPromptModel) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if model == nil {
		return nil
	}
	// An armed discard owns every input, paste included: the question on screen
	// is whether to throw this text away, and letting a paste answer it would
	// change the text being asked about.
	if model.confirm.IsActive() {
		return nil
	}
	model.notice = ""
	var cmd tea.Cmd
	if model.options.Multiline {
		model.area, cmd = model.area.Update(msg)
	} else {
		model.input, cmd = model.input.Update(msg)
	}
	return cmd
}

// submit is the one road out of the buffer. It trims exactly what the
// application layer trims, so a buffer holding only whitespace is refused here
// with the text still on screen rather than committed and rejected.
func (model *OperationTextPromptModel) submit() (tea.Cmd, bool, tea.Msg) {
	if model.submitted {
		return nil, false, nil
	}
	value := strings.TrimSpace(model.bufferValue())
	if value == "" && !model.options.AcceptsEmpty {
		model.notice = model.options.EmptyNotice
		return nil, false, nil
	}
	model.submitted = true
	return nil, true, operation.OperationValueSelectedMsg{
		Request: model.request,
		Value:   value,
	}
}

func (model *OperationTextPromptModel) Render(width int) string {
	if model == nil {
		return ""
	}
	innerWidth := min(max(width*3/5, 46), max(width-6, 24))
	bufferWidth := max(innerWidth-4, 10)

	var content strings.Builder
	if model.subtitle != "" {
		label := model.options.SubtitleLabel
		if label == "" {
			label = "Current"
		}
		content.WriteString(theme.MutedStyle.Render(
			label + ": " + textfit.TruncateToVisualWidth(model.subtitle, max(innerWidth-len(label)-4, 8)),
		))
		content.WriteString("\n\n")
	}
	if model.options.Multiline {
		model.area.SetWidth(bufferWidth)
		content.WriteString(model.area.View())
	} else {
		model.input.SetWidth(bufferWidth)
		content.WriteString(model.input.View())
	}
	// The discard question is drawn inside this box rather than as its own
	// overlay: the shell paints the confirmation it owns, not one a value step
	// armed for itself, and a second floating box over this one would hide the
	// text the question is about.
	if model.confirm.IsActive() {
		content.WriteString("\n\n")
		content.WriteString(theme.WarningIconStyle.Render(model.confirm.Prompt()))
	}
	if model.notice != "" {
		content.WriteString("\n\n")
		content.WriteString(theme.ValidationErrorStyle.Render(model.notice))
	}

	return chrome.RenderOverlayBox(
		content.String(),
		innerWidth,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(model.title),
		chrome.RenderGuidanceText(model.guidanceSpec(), max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

// guidanceSpec names the keys that actually apply right now, which is why the
// submit key is read from the same place HandleKey reads it.
func (model *OperationTextPromptModel) guidanceSpec() chrome.GuidanceSpec {
	if model.confirm.IsActive() {
		return chrome.ConfirmationGuidanceSpec()
	}
	apply := "Enter apply"
	if model.options.Multiline {
		apply = "Ctrl+S save"
	}
	return chrome.GuidanceSpec{
		Hints: chrome.RequiredGuidanceHints(apply, "Esc back"),
		Rows:  2,
	}
}
