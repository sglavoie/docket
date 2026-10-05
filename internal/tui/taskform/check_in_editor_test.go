package taskform

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/util"
)

func checkInEditorKey(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
	}
}

func TestBlankBulkCheckInEditorRequiresAnExplicitReplacement(t *testing.T) {
	model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Replace schedule on 2 selected tasks", models.CheckInSchedule{}, true)
	_, dismissed, result := model.HandleKey(checkInEditorKey("ctrl+s"))
	if dismissed || result != nil {
		t.Fatalf("blank save dismissed=%v result=%T, want retained draft", dismissed, result)
	}
	if !strings.Contains(model.Notice(), "Choose a next check-in date or select Clear") {
		t.Fatalf("notice = %q", model.Notice())
	}
	if !strings.Contains(model.Render(100), "Replace schedule on 2 selected tasks") {
		t.Fatal("bulk editor does not label replacement semantics")
	}
}

func TestCheckInEditorPresetsStayDraftedUntilSave(t *testing.T) {
	model := NewCheckInEditorModel(operation.OperationExecutionRequest{ID: operation.OperationTaskSetCheckIn}, "Task", models.CheckInSchedule{}, true)

	// Enter opens the date picker. Choosing tomorrow changes only the draft.
	_, dismissed, result := model.HandleKey(checkInEditorKey("enter"))
	if dismissed || result != nil {
		t.Fatal("opening the date picker submitted the operation")
	}
	model.HandleKey(checkInEditorKey("down"))
	model.HandleKey(checkInEditorKey("enter"))
	model.HandleKey(checkInEditorKey("tab"))
	model.HandleKey(checkInEditorKey("enter"))
	for range 3 { // Off -> 1 -> 3 -> 7.
		model.HandleKey(checkInEditorKey("down"))
	}
	model.HandleKey(checkInEditorKey("enter"))

	_, dismissed, result = model.HandleKey(checkInEditorKey("ctrl+s"))
	selected, ok := result.(operation.OperationValueSelectedMsg)
	if !dismissed || !ok {
		t.Fatalf("save dismissed=%v result=%T", dismissed, result)
	}
	value, ok := selected.Value.(operation.OperationCheckInValue)
	if !ok || value.Clear || value.Schedule.Next == nil || value.Schedule.RepeatDays != 7 {
		t.Fatalf("selected value = %#v", selected.Value)
	}
}

func TestCheckInEditorCustomValidationAndNestedCancelPreserveDraft(t *testing.T) {
	model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{}, true)
	model.dateInput.SetValue("2026-09-20")
	model.repeatInput.SetValue("-3")
	model.explicit = true

	// Escape closes only the nested picker and leaves the editor and its fields.
	model.HandleKey(checkInEditorKey("enter"))
	_, dismissed, result := model.HandleKey(checkInEditorKey("esc"))
	if dismissed || result != nil || model.DateInput() != "2026-09-20" {
		t.Fatalf("nested cancel dismissed=%v result=%T date=%q", dismissed, result, model.DateInput())
	}
	_, dismissed, result = model.HandleKey(checkInEditorKey("ctrl+s"))
	if dismissed || result != nil || !strings.Contains(model.Notice(), "cannot be negative") {
		t.Fatalf("invalid repeat dismissed=%v result=%T notice=%q", dismissed, result, model.Notice())
	}
	if model.DateInput() != "2026-09-20" || model.RepeatInput() != "-3" {
		t.Fatal("validation failure changed the draft")
	}

	model.repeatInput.SetValue("45")
	_, dismissed, result = model.HandleKey(checkInEditorKey("ctrl+s"))
	selected, ok := result.(operation.OperationValueSelectedMsg)
	if !dismissed || !ok {
		t.Fatalf("custom save dismissed=%v result=%T", dismissed, result)
	}
	value := selected.Value.(operation.OperationCheckInValue)
	want := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	if value.Schedule.Next == nil || value.Schedule.Next.Format(time.DateOnly) != want.Format(time.DateOnly) || value.Schedule.RepeatDays != 45 {
		t.Fatalf("custom schedule = %+v", value.Schedule)
	}
}

func TestCheckInEditorClearIsExplicitAndOuterEscapeCancels(t *testing.T) {
	day := time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local)
	model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{Next: &day, RepeatDays: 14}, false)
	if model.DateInput() != "2026-09-20" || model.RepeatInput() != "14" {
		t.Fatalf("single prefill = %q/%q", model.DateInput(), model.RepeatInput())
	}
	model.HandleKey(checkInEditorKey("enter"))
	for range len(checkInDatePresets) - 1 {
		model.HandleKey(checkInEditorKey("down"))
	}
	model.HandleKey(checkInEditorKey("enter"))
	_, dismissed, result := model.HandleKey(checkInEditorKey("ctrl+s"))
	selected := result.(operation.OperationValueSelectedMsg)
	value := selected.Value.(operation.OperationCheckInValue)
	if !dismissed || !value.Clear || !value.Schedule.IsEmpty() {
		t.Fatalf("clear value = %+v dismissed=%v", value, dismissed)
	}

	cancel := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{Next: &day}, false)
	_, dismissed, result = cancel.HandleKey(checkInEditorKey("esc"))
	if !dismissed {
		t.Fatal("outer Escape did not cancel the editor")
	}
	if _, ok := result.(operation.OperationValueCanceledMsg); !ok {
		t.Fatalf("outer Escape result = %T", result)
	}
}

func TestCheckInEditorCalendarDatePresets(t *testing.T) {
	loc, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	previous := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = previous })
	for _, tc := range []struct {
		label string
		from  time.Time
	}{
		{"today", time.Date(2026, 3, 8, 12, 0, 0, 0, loc)},
		{"tomorrow", time.Date(2026, 3, 7, 12, 0, 0, 0, loc)},
		{"+week", time.Date(2026, 3, 1, 12, 0, 0, 0, loc)},
		{"+30 days", time.Date(2026, 2, 6, 12, 0, 0, 0, loc)},
	} {
		t.Run(tc.label, func(t *testing.T) {
			model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{}, true)
			model.now = func() time.Time { return tc.from }
			model.HandleKey(checkInEditorKey("enter"))
			for _, preset := range checkInDatePresets {
				if preset.label == tc.label {
					break
				}
				model.HandleKey(checkInEditorKey("down"))
			}
			model.HandleKey(checkInEditorKey("enter"))
			if model.DateInput() != "2026-03-08" {
				t.Fatalf("preset drafted %q, want 2026-03-08", model.DateInput())
			}
			_, dismissed, result := model.HandleKey(checkInEditorKey("ctrl+s"))
			selected, ok := result.(operation.OperationValueSelectedMsg)
			if !dismissed || !ok {
				t.Fatalf("save dismissed=%v result=%T notice=%q", dismissed, result, model.Notice())
			}
			value := selected.Value.(operation.OperationCheckInValue)
			if value.Schedule.Next == nil || value.Schedule.Next.Format(time.DateOnly) != "2026-03-08" {
				t.Fatalf("saved schedule = %+v", value.Schedule)
			}
			if err := models.ValidateCheckInSchedule(value.Schedule); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckInEditorRefusesSkippedPresetWithoutLosingDraft(t *testing.T) {
	loc, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatal(err)
	}
	model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{}, true)
	model.now = func() time.Time { return time.Date(2011, 12, 29, 12, 0, 0, 0, loc) }
	model.dateInput.SetValue("2011-12-31")
	model.repeatInput.SetValue("7")
	model.HandleKey(checkInEditorKey("enter"))
	model.HandleKey(checkInEditorKey("down"))
	_, dismissed, result := model.HandleKey(checkInEditorKey("enter"))
	if dismissed || result != nil || model.Notice() == "" || model.picker != checkInPickerDate {
		t.Fatalf("refusal dismissed=%v result=%T notice=%q picker=%v", dismissed, result, model.Notice(), model.picker)
	}
	if model.DateInput() != "2011-12-31" || model.RepeatInput() != "7" {
		t.Fatal("refused preset changed the draft")
	}
}

func TestCheckInRelativeDatePassesScheduleValidation(t *testing.T) {
	loc, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input string
		day   int
	}{{"tomorrow", 7}, {"+7d", 1}, {"next week", 1}} {
		next, err := util.ParseCheckInDateAt(tc.input, time.Date(2026, 3, tc.day, 12, 0, 0, 0, loc))
		if err != nil {
			t.Fatal(err)
		}
		if next.Format(time.DateOnly) != "2026-03-08" {
			t.Fatalf("%q = %s, want 2026-03-08", tc.input, next)
		}
		if err := models.ValidateCheckInSchedule(models.CheckInSchedule{Next: &next}); err != nil {
			t.Fatalf("%q: %v", tc.input, err)
		}
	}
}

func TestCheckInRepeatPresetsPreserveCurrentInterval(t *testing.T) {
	day := time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local)
	for _, days := range []int{0, 1, 3, 7, 14, 30, 45} {
		model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{Next: &day, RepeatDays: days}, false)
		model.HandleKey(checkInEditorKey("tab"))
		model.HandleKey(checkInEditorKey("enter"))
		model.HandleKey(checkInEditorKey("enter"))
		_, dismissed, result := model.HandleKey(checkInEditorKey("ctrl+s"))
		selected, ok := result.(operation.OperationValueSelectedMsg)
		if !dismissed || !ok {
			t.Fatalf("interval %d: save failed: %s", days, model.Notice())
		}
		if got := selected.Value.(operation.OperationCheckInValue).Schedule.RepeatDays; got != days {
			t.Fatalf("opening and accepting presets changed %d to %d", days, got)
		}
	}
}

func TestCheckInDatePreviewMatchesSavedRelativeDate(t *testing.T) {
	model := NewCheckInEditorModel(operation.OperationExecutionRequest{}, "Task", models.CheckInSchedule{}, true)
	model.now = func() time.Time { return time.Date(2026, 1, 31, 12, 0, 0, 0, time.Local) }
	model.HandlePaste(tea.PasteMsg{Content: "+30d"})
	model.repeatInput.SetValue("7")
	rendered := model.Render(60)
	if !strings.Contains(rendered, "Mon Mar 2, 2026") || !strings.Contains(rendered, "7 days after you acknowledge.") {
		t.Fatalf("missing date or repeat preview: %s", rendered)
	}
	model.HandleKey(checkInEditorKey("enter"))
	if rendered := model.Render(60); !strings.Contains(rendered, "+30 days · Mon Mar 2, 2026") || strings.Contains(rendered, "+month") {
		t.Fatalf("preset hides its interval or resolved date: %s", rendered)
	}
	model.HandleKey(checkInEditorKey("esc"))
	_, _, result := model.HandleKey(checkInEditorKey("ctrl+s"))
	value := result.(operation.OperationValueSelectedMsg).Value.(operation.OperationCheckInValue)
	if got := value.Schedule.Next.Format(time.DateOnly); got != "2026-03-02" {
		t.Fatalf("saved date %s differs from preview", got)
	}
	model.Restore(value, "")
	model.dateInput.SetValue("nonsense")
	if strings.Contains(model.Render(60), "Resolves to") {
		t.Fatal("invalid input kept a stale date preview")
	}
}
