package taskform

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
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
	{label: "+month", days: 30},
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

	dateInput    textinput.Model
	repeatInput  textinput.Model
	focus        int
	picker       checkInPicker
	pickerCursor int
	notice       string
	clear        bool
	explicit     bool
	submitted    bool
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
	date.Placeholder = "today, tomorrow, +7d, or YYYY-MM-DD"
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
	}
	if !bulk {
		if schedule.Next != nil {
			model.dateInput.SetValue(schedule.Next.Format(time.DateOnly))
		}
		model.repeatInput.SetValue(repeatDisplay(schedule.RepeatDays))
		model.explicit = schedule.Next != nil
	}
	model.syncFocus()
	return model
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
		length = len(checkInRepeatPresets)
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
				model.clear = false
				day := util.StartOfDay(time.Now()).AddDate(0, 0, preset.days)
				model.dateInput.SetValue(day.Format(time.DateOnly))
			}
		} else {
			model.clear = false
			model.repeatInput.SetValue(repeatDisplay(checkInRepeatPresets[model.pickerCursor]))
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
	next, err := util.ParseCheckInDate(dateInput)
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
	innerWidth := min(max(width*3/5, 52), max(width-6, 28))
	contentWidth := max(innerWidth-4, 16)
	var content strings.Builder
	if model.subtitle != "" {
		content.WriteString(theme.MutedStyle.Render(model.subtitle))
		content.WriteString("\n\n")
	}
	if model.picker == checkInPickerDate {
		content.WriteString(model.renderDatePicker(contentWidth))
	} else if model.picker == checkInPickerRepeat {
		content.WriteString(model.renderRepeatPicker(contentWidth))
	} else {
		model.dateInput.SetWidth(max(contentWidth-18, 10))
		model.repeatInput.SetWidth(max(contentWidth-18, 10))
		theme.ApplyThemedTextInput(&model.dateInput)
		theme.ApplyThemedTextInput(&model.repeatInput)
		content.WriteString(chrome.RenderFormLabelWithWidth("Next check-in:", 17, model.focus == checkInFieldNext))
		content.WriteString(model.dateInput.View())
		content.WriteString("\n")
		content.WriteString(chrome.RenderFormLabelWithWidth("Repeat:", 17, model.focus == checkInFieldRepeat))
		content.WriteString(model.repeatInput.View())
		content.WriteString("\n\n")
		content.WriteString(theme.MutedStyle.Render("Repeat options: Off, 1, 3, 7, 14, 30, or custom days"))
	}
	if model.notice != "" {
		content.WriteString("\n\n")
		content.WriteString(theme.ValidationErrorStyle.Render(model.notice))
	}
	return chrome.RenderOverlayBox(
		content.String(), innerWidth,
		lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render(model.title),
		chrome.RenderGuidanceText(model.guidanceSpec(), contentWidth),
		lipgloss.NewStyle().Foreground(theme.ColorPrimary),
	)
}

func (model *CheckInEditorModel) renderDatePicker(width int) string {
	var b strings.Builder
	for i, preset := range checkInDatePresets {
		line := "  " + preset.label
		if i == model.pickerCursor {
			line = theme.SelectedStyle.Render(fmt.Sprintf(" %-*s", max(width-2, 1), preset.label))
		}
		b.WriteString(line)
		if i < len(checkInDatePresets)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (model *CheckInEditorModel) renderRepeatPicker(width int) string {
	var b strings.Builder
	for i, days := range checkInRepeatPresets {
		label := repeatDisplay(days)
		if days > 0 {
			label += " days"
		}
		line := "  " + label
		if i == model.pickerCursor {
			line = theme.SelectedStyle.Render(fmt.Sprintf(" %-*s", max(width-2, 1), label))
		}
		b.WriteString(line)
		if i < len(checkInRepeatPresets)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (model *CheckInEditorModel) guidanceSpec() chrome.GuidanceSpec {
	if model.picker != checkInPickerNone {
		return chrome.RequiredActionGuidanceSpec("Enter select", "j/k navigate", "Esc back")
	}
	return chrome.RequiredActionGuidanceSpec("Ctrl+S save", "Tab/Shift+Tab fields", "Enter presets", "Esc cancel")
}
