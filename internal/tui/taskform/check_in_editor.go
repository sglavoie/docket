package taskform

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

const (
	checkInFieldNext = iota
	checkInFieldRepeat
)

type checkInPicker uint8

const (
	checkInPickerNone checkInPicker = iota
	checkInPickerDate
	checkInPickerRepeat
)

type checkInPreset struct {
	label string
	days  int
	clear bool
}

var checkInDatePresets = []checkInPreset{
	{label: "today"},
	{label: "tomorrow", days: 1},
	{label: "+week", days: 7},
	{label: "+30 days", days: 30},
	{label: "clear", clear: true},
}

var checkInRepeatPresets = []int{0, 1, 3, 7, 14, 30}

// CheckInEditorModel is the compact shared editor for one check-in schedule.
// The operation request already contains frozen project-qualified identities;
// this model only owns the draft that will be applied to those identities.
type CheckInEditorModel struct {
	request  operation.OperationExecutionRequest
	title    string
	subtitle string

	initialDate, initialRepeat string
	dateInput                  textinput.Model
	repeatInput                textinput.Model
	focus                      int
	picker                     checkInPicker
	pickerCursor               int
	notice                     string
	clear                      bool
	explicit                   bool
	submitted                  bool
	setup                      bool
	now                        func() time.Time
}

// NewCheckInEditorModel creates either a prefilled single-task editor or a
// blank bulk replacement editor. Bulk never borrows one selected task's value.
func NewCheckInEditorModel(
	request operation.OperationExecutionRequest,
	subtitle string,
	schedule models.CheckInSchedule,
	bulk bool,
) *CheckInEditorModel {
	date := textinput.New()
	date.Placeholder = "today, +7d, or YYYY-MM-DD"
	date.CharLimit = 64
	repeat := textinput.New()
	repeat.Placeholder = "Off or positive days"
	repeat.CharLimit = 8

	model := &CheckInEditorModel{
		request:     request,
		title:       "Set Check-In",
		subtitle:    subtitle,
		dateInput:   date,
		repeatInput: repeat,
		now:         time.Now,
	}
	if !bulk {
		if schedule.Next != nil {
			model.dateInput.SetValue(schedule.Next.Format(time.DateOnly))
		}
		model.repeatInput.SetValue(repeatDisplay(schedule.RepeatDays))
		model.explicit = schedule.Next != nil
	}
	model.initialDate, model.initialRepeat = model.dateInput.Value(), model.repeatInput.Value()
	model.syncFocus()
	return model
}

// ConfigureSetup supplies the guided pass's title and modal key guidance. The
// shell owns Skip and Escape, while the two-field draft remains shared.
func (model *CheckInEditorModel) ConfigureSetup() {
	model.setup = true
	model.title = "Set Up Check-Ins"
}

func repeatDisplay(days int) string {
	if days == models.CheckInRepeatOff {
		return "Off"
	}
	return strconv.Itoa(days)
}

func (model *CheckInEditorModel) syncFocus() {
	model.dateInput.Blur()
	model.repeatInput.Blur()
	if model.focus == checkInFieldNext {
		model.dateInput.Focus()
	} else {
		model.repeatInput.Focus()
	}
}

func (model *CheckInEditorModel) InitCmd() tea.Cmd { return textinput.Blink }

// Restore reopens an asynchronously refused save with the submitted draft.
func (model *CheckInEditorModel) Restore(value operation.OperationCheckInValue, notice string) {
	if model == nil {
		return
	}
	model.dateInput.SetValue(value.DateInput)
	model.repeatInput.SetValue(value.RepeatInput)
	model.clear = value.Clear
	model.explicit = true
	model.submitted = false
	model.notice = notice
	model.picker = checkInPickerNone
	model.syncFocus()
}

func (model *CheckInEditorModel) DateInput() string {
	if model == nil {
		return ""
	}
	return model.dateInput.Value()
}

func (model *CheckInEditorModel) RepeatInput() string {
	if model == nil {
		return ""
	}
	return model.repeatInput.Value()
}

func (model *CheckInEditorModel) Notice() string {
	if model == nil {
		return ""
	}
	return model.notice
}

func (model *CheckInEditorModel) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if model == nil || model.picker != checkInPickerNone || model.submitted {
		return nil
	}
	model.notice = ""
	model.clear = false
	model.explicit = true
	var cmd tea.Cmd
	if model.focus == checkInFieldNext {
		model.dateInput, cmd = model.dateInput.Update(msg)
	} else {
		model.repeatInput, cmd = model.repeatInput.Update(msg)
	}
	return cmd
}

func (model *CheckInEditorModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil {
		return nil, true, operation.OperationValueCanceledMsg{}
	}
	if model.picker != checkInPickerNone {
		return model.handlePickerKey(msg)
	}
	key := msg.String()
	switch key {
	case "esc":
		return nil, true, operation.OperationValueCanceledMsg{}
	case "tab":
		model.focus = (model.focus + 1) % 2
		model.syncFocus()
		return nil, false, nil
	case "shift+tab":
		model.focus = (model.focus + 1) % 2
		model.syncFocus()
		return nil, false, nil
	case "enter":
		model.pickerCursor = 0
		if model.focus == checkInFieldNext {
			model.picker = checkInPickerDate
		} else {
			model.picker = checkInPickerRepeat
			current, _ := strconv.Atoi(strings.TrimSpace(model.repeatInput.Value()))
			for i, days := range model.repeatPresets() {
				if days == current {
					model.pickerCursor = i
					break
				}
			}
		}
		return nil, false, nil
	case "ctrl+s":
		return model.submit()
	default:
		if model.submitted {
			return nil, false, nil
		}
		model.notice = ""
		model.clear = false
		model.explicit = true
		var cmd tea.Cmd
		if model.focus == checkInFieldNext {
			model.dateInput, cmd = model.dateInput.Update(msg)
		} else {
			model.repeatInput, cmd = model.repeatInput.Update(msg)
		}
		return cmd, false, nil
	}
}

func (model *CheckInEditorModel) handlePickerKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	length := len(checkInDatePresets)
	if model.picker == checkInPickerRepeat {
		length = len(model.repeatPresets())
	}
	switch msg.String() {
	case "esc":
		model.picker = checkInPickerNone
		return nil, false, nil
	case "up", "k":
		if model.pickerCursor > 0 {
			model.pickerCursor--
		}
		return nil, false, nil
	case "down", "j":
		if model.pickerCursor < length-1 {
			model.pickerCursor++
		}
		return nil, false, nil
	case "enter":
		model.notice = ""
		model.explicit = true
		if model.picker == checkInPickerDate {
			preset := checkInDatePresets[model.pickerCursor]
			if preset.clear {
				model.clear = true
				model.dateInput.SetValue("")
				model.repeatInput.SetValue("Off")
			} else {
				day, err := util.AddCalendarDays(model.now(), preset.days)
				if err != nil {
					model.notice = err.Error()
					return nil, false, nil
				}
				model.clear = false
				model.dateInput.SetValue(day.Format(time.DateOnly))
			}
		} else {
			model.clear = false
			model.repeatInput.SetValue(repeatDisplay(model.repeatPresets()[model.pickerCursor]))
		}
		model.picker = checkInPickerNone
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

func (model *CheckInEditorModel) submit() (tea.Cmd, bool, tea.Msg) {
	if model.submitted {
		return nil, false, nil
	}
	dateInput := strings.TrimSpace(model.dateInput.Value())
	repeatInput := strings.TrimSpace(model.repeatInput.Value())
	value := operation.OperationCheckInValue{
		Clear: model.clear, DateInput: dateInput, RepeatInput: repeatInput,
	}
	if model.clear {
		model.submitted = true
		return nil, true, operation.OperationValueSelectedMsg{Request: model.request, Value: value}
	}
	if !model.explicit || dateInput == "" {
		model.notice = "Choose a next check-in date or select Clear."
		return nil, false, nil
	}
	next, err := util.ParseCheckInDateAt(dateInput, model.now())
	if err != nil {
		model.notice = err.Error()
		return nil, false, nil
	}
	repeatDays := models.CheckInRepeatOff
	if repeatInput != "" && !strings.EqualFold(repeatInput, "off") {
		repeatDays, err = strconv.Atoi(repeatInput)
		if err != nil {
			model.notice = "Repeat must be Off or a positive day count."
			return nil, false, nil
		}
	}
	value.Schedule = models.CheckInSchedule{Next: &next, RepeatDays: repeatDays}
	if err := models.ValidateCheckInSchedule(value.Schedule); err != nil {
		model.notice = err.Error()
		return nil, false, nil
	}
	model.submitted = true
	return nil, true, operation.OperationValueSelectedMsg{Request: model.request, Value: value}
}

func (model *CheckInEditorModel) Render(width int) string {
	if model == nil {
		return ""
	}
	innerWidth := formOverlayInnerWidth(width, max(width*3/5, 52), 28)
	contentWidth := max(innerWidth-4, 16)
	var content strings.Builder
	if model.subtitle != "" {
		// The single-task subtitle is that task's whole title, so it is wrapped
		// rather than left to widen the box past the frame it was sized for.
		content.WriteString(theme.MutedStyle.Render(
			strings.Join(textfit.WrapText(model.subtitle, contentWidth), "\n")))
		content.WriteString("\n\n")
	}
	switch model.picker {
	case checkInPickerDate:
		content.WriteString(model.renderDatePicker(contentWidth))
	case checkInPickerRepeat:
		content.WriteString(model.renderRepeatPicker(contentWidth))
	default:
		// The focused marker and its separating space consume two columns before
		// the 17-column label, and the text input's own prompt consumes two more.
		// Giving the input the true remainder prevents the creation form's
		// minimum-width frame from wrapping and losing its bottom border.
		model.dateInput.SetWidth(max(contentWidth-21, 10))
		model.repeatInput.SetWidth(max(contentWidth-21, 10))
		theme.ApplyThemedTextInput(&model.dateInput)
		theme.ApplyThemedTextInput(&model.repeatInput)
		content.WriteString(chrome.RenderFormLabelWithWidth("Next check-in:", 17, model.focus == checkInFieldNext))
		content.WriteString(model.dateInput.View())
		if day, err := util.ParseCheckInDateAt(model.dateInput.Value(), model.now()); err == nil && !model.clear {
			content.WriteString("\n" + theme.MutedStyle.Render("Resolves to "+chrome.FormatWeekdayDate(day, model.now(), false)))
		}
		content.WriteString("\n")
		content.WriteString(chrome.RenderFormLabelWithWidth("Repeat:", 17, model.focus == checkInFieldRepeat))
		content.WriteString(model.repeatInput.View())
		content.WriteString("\n\n")
		hint := "Repeat: Off, 1, 3, 7, 14, 30, or custom days"
		if days, err := strconv.Atoi(strings.TrimSpace(model.repeatInput.Value())); err == nil && days > 0 && models.ValidateCheckInRepeatDays(days) == nil {
			unit := "days"
			if days == 1 {
				unit = "day"
			}
			hint = fmt.Sprintf("Next reminder: %d %s after you acknowledge.", days, unit)
		}
		content.WriteString(theme.MutedStyle.Render(strings.Join(textfit.WrapText(hint, contentWidth), "\n")))
	}
	if model.notice != "" {
		content.WriteString("\n\n")
		content.WriteString(theme.ValidationErrorStyle.Render(model.notice))
	}
	title := model.title
	if model.setup && model.submitted {
		title += " · Saving…"
	}
	return chrome.RenderOverlayBox(
		content.String(), innerWidth,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(title),
		chrome.RenderGuidanceText(model.guidanceSpec(), contentWidth),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

func (model *CheckInEditorModel) renderDatePicker(width int) string {
	var b strings.Builder
	now := model.now()
	for i, preset := range checkInDatePresets {
		label := preset.label
		if !preset.clear {
			if day, err := util.AddCalendarDays(now, preset.days); err == nil {
				label += " · " + chrome.FormatWeekdayDate(day, now, false)
			}
		}
		line := "  " + label
		if i == model.pickerCursor {
			line = theme.SelectedStyle.Render(fmt.Sprintf(" %-*s", max(width-2, 1), label))
		}
		b.WriteString(line)
		if i < len(checkInDatePresets)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Keep custom intervals selectable so opening and accepting presets preserves them.
func (model *CheckInEditorModel) repeatPresets() []int {
	days, err := strconv.Atoi(strings.TrimSpace(model.repeatInput.Value()))
	if err == nil && days > 0 && models.ValidateCheckInRepeatDays(days) == nil {
		for _, preset := range checkInRepeatPresets {
			if preset == days {
				return checkInRepeatPresets
			}
		}
		return append([]int{days}, checkInRepeatPresets...)
	}
	return checkInRepeatPresets
}

func (model *CheckInEditorModel) renderRepeatPicker(width int) string {
	var b strings.Builder
	presets := model.repeatPresets()
	for i, days := range presets {
		label := repeatDisplay(days)
		if days > 0 {
			label += " days"
		}
		line := "  " + label
		if i == model.pickerCursor {
			line = theme.SelectedStyle.Render(fmt.Sprintf(" %-*s", max(width-2, 1), label))
		}
		b.WriteString(line)
		if i < len(presets)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (model *CheckInEditorModel) guidanceSpec() chrome.GuidanceSpec {
	if model.setup {
		if model.submitted {
			return chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("Esc exit after save")}
		}
		if model.picker != checkInPickerNone {
			return chrome.GuidanceSpec{Hints: append(chrome.RequiredGuidanceHints("Enter select", "Ctrl+N skip", "Esc exit"), chrome.OptionalGuidanceHints(10, "j/k navigate")...)}
		}
		return chrome.GuidanceSpec{Hints: append(chrome.RequiredGuidanceHints("Ctrl+S save", "Ctrl+N skip", "Esc exit"), chrome.OptionalGuidanceHints(10, "Enter presets", "Tab fields")...)}
	}
	if model.picker != checkInPickerNone {
		return chrome.GuidanceSpec{Hints: append(chrome.RequiredGuidanceHints("Enter select", "Esc back"), chrome.OptionalGuidanceHints(10, "j/k navigate")...)}
	}
	return chrome.GuidanceSpec{Hints: append(chrome.RequiredGuidanceHints("Ctrl+S save", "Esc cancel"), chrome.OptionalGuidanceHints(10, "Tab/Shift+Tab fields", "Enter presets")...)}
}

// QuitBlockReason names content a desktop quit would abandon.
func (model *CheckInEditorModel) QuitBlockReason() string {
	if model.submitted {
		return "Your check-in is still being saved."
	}
	if model.dateInput.Value() != model.initialDate || model.repeatInput.Value() != model.initialRepeat || model.clear {
		return "Your check-in changes have not been saved."
	}
	return ""
}
