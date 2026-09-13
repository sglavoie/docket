package taskform

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
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
